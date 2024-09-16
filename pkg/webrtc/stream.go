package webrtc

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	ws "pinzoom/pkg/websocket"
	"sync"

	"github.com/pion/webrtc/v3"
)

func StreamConn(c *ws.WebSocket, p *Peers) error {
	var config webrtc.Configuration
	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		config = turnConfig
	}
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}
	defer func(peerConnection *webrtc.PeerConnection) {
		if err := peerConnection.Close(); err != nil {
			logrus.Errorf("unexpected error while closing peerConnection, err=%v", err)
			return
		}
	}(peerConnection)

	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			return fmt.Errorf("unexpected error while creating a new RtpTransceiver, err=%v", err)
		}
	}

	newPeer := PeerConnectionState{
		PeerConnection: peerConnection,
		Websocket: &ThreadSafeWriter{
			Conn:  c,
			Mutex: sync.Mutex{},
		}}

	p.ListLock.Lock()
	p.Connections = append(p.Connections, newPeer)
	p.ListLock.Unlock()

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}

		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			logrus.Errorf("failed to marshal iceCandidate on stream, err=%v", err)
			return
		}

		if writeErr := newPeer.Websocket.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			logrus.Errorf("failed to write JSON into WS on stream, err=%v", writeErr)
		}
	})

	peerConnection.OnConnectionStateChange(func(pp webrtc.PeerConnectionState) {
		switch pp {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				logrus.Errorf("failed to close stream peerConnection, err=%v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			p.SignalPeerConnections()
		default:
			logrus.Error("unhandled default case")
		}
	})

	p.SignalPeerConnections()
	message := &websocketMessage{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			return err
		} else if err := json.Unmarshal(raw, &message); err != nil {
			return err
		}

		switch message.Event {
		case "candidate":
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				return err
			}

			if err := peerConnection.AddICECandidate(candidate); err != nil {
				return err
			}
		case "answer":
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				return err
			}

			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				return err
			}
		}
	}
}
