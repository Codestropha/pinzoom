package webrtc

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"os"
	"pinzoom/pkg/hub"
	"sync"

	"github.com/pion/webrtc/v3"
)

func RoomConn(ctx *hub.Ctx, p *Peers) error {
	var config webrtc.Configuration
	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		config = turnConfig
	}
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}
	defer peerConnection.Close()

	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			return err
		}
	}

	newPeer := PeerConnectionState{
		PeerConnection: peerConnection,
		Websocket: &ThreadSafeWriter{
			Conn:  ctx.WebSocket,
			Mutex: sync.Mutex{},
		}}

	// Add our new PeerConnection to global list
	p.ListLock.Lock()
	p.Connections = append(p.Connections, newPeer)
	p.ListLock.Unlock()

	// Trickle ICE. Emit server candidate to client
	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}

		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			logrus.Errorf("failed to marshal iceCandidate in room, err=%v", err)
			return
		}

		if writeErr := newPeer.Websocket.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			logrus.Errorf("failed to write JSON into WS in room, err=%v", writeErr)
		}
	})

	// If PeerConnection is closed remove it from global list
	peerConnection.OnConnectionStateChange(func(pp webrtc.PeerConnectionState) {
		switch pp {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				logrus.Errorf("failed to close room peerConnection, err=%v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			p.SignalPeerConnections()
		default:
			logrus.Error("unhandled default case")
		}
	})

	peerConnection.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		// Create a track to fan out our incoming video to all peers
		trackLocal := p.AddTrack(t)
		if trackLocal == nil {
			return
		}
		defer p.RemoveTrack(trackLocal)

		buf := make([]byte, 1500)
		for {
			i, _, err := t.Read(buf)
			if err != nil {
				return
			}

			if _, err = trackLocal.Write(buf[:i]); err != nil {
				return
			}
		}
	})

	p.SignalPeerConnections()
	message := &websocketMessage{}
	for {
		_, raw, err := ctx.WebSocket.ReadMessage()
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
