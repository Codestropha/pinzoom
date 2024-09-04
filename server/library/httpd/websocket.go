package httpd

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"pinzoom/library/errs"
	"pinzoom/library/httpd/websocket"
	"sync"
)

type WebSocket struct {
	ctx  context.Context
	r    *http.Request
	conn *websocket.Conn

	close  chan struct{}
	closed chan struct{}

	incoming chan []byte
	outgoing chan []byte

	mu        sync.Mutex // Guards listeners.
	listeners []WebSocketListener
}

type WebSocketListener interface {
	OnWebSocketOpened(ws *WebSocket)
	OnWebSocketClosed(ws *WebSocket)
}

func NewWebSocket(ctx context.Context, w http.ResponseWriter, r *http.Request) (*WebSocket, error) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: false,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	ws := &WebSocket{
		ctx:      ctx,
		r:        r,
		conn:     conn,
		close:    make(chan struct{}, 1),
		closed:   make(chan struct{}),
		incoming: make(chan []byte, 8),
		outgoing: make(chan []byte, 8),
	}

	go ws.mainLoop()
	go ws.readLoop()
	return ws, nil
}

func (ws *WebSocket) Closed() <-chan struct{} { return ws.closed }
func (ws *WebSocket) Incoming() <-chan []byte { return ws.incoming }
func (ws *WebSocket) Outgoing() chan<- []byte { return ws.outgoing }

func (ws *WebSocket) Send(ctx context.Context, text string) {
	select {
	case ws.outgoing <- []byte(text):
	case <-ctx.Done():
	}
}

func (ws *WebSocket) SendJSON(ctx context.Context, v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}

	select {
	case ws.outgoing <- bytes:
	case <-ctx.Done():
	}
	return nil
}

func (ws *WebSocket) Close() <-chan struct{} {
	select {
	case ws.close <- struct{}{}:
	default:
	}
	return ws.closed
}

func (ws *WebSocket) mainLoop() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Debug(ws.ctx, "Stack in a WebSocket main loop", err)
		}
	}()

	defer ws.onClosed()
	defer close(ws.closed)
	defer close(ws.incoming)
	defer ws.conn.Close()

	logrus.WithContext(ws.ctx).Infof("WS %v", ws.r.RequestURI)
	defer logrus.WithContext(ws.ctx).Debug("WS END")

	for {
		select {
		case <-ws.close:
			return
		case msg := <-ws.outgoing:
			if err := ws.sendMessageOrRecover(msg); err != nil {
				logrus.WithContext(ws.ctx).Debug("WebSocket failed to send a message", err)
				return
			}
		}
	}
}

func (ws *WebSocket) readLoop() {
	defer func() {
		ws.Close()
		if err := recover(); err != nil {
			logrus.WithContext(ws.ctx).Debug("WebSocket panic in a read loop", err)
		}
	}()

	for {
		_, msg, err := ws.conn.ReadMessage()
		if err != nil {
			return
		}
		logrus.WithContext(ws.ctx).Debugf("WebSocket incoming message, len=%d", len(msg))

		select {
		case ws.incoming <- msg:
		case <-ws.closed:
		}
	}
}

func (ws *WebSocket) sendMessageOrRecover(msg []byte) (err error) {
	defer func() {
		if e := recover(); e != nil {
			logrus.WithContext(ws.ctx).Debug("Stack in an SSEStream send method", e)
			err = errs.Recovered(e)
		}
	}()

	w, err := ws.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = w.Write(msg)
	logrus.WithContext(ws.ctx).Tracef("WebSocket sent a message, len=%d", len(msg))
	return err
}

func (ws *WebSocket) onClosed() {
	listeners := ws.copyListeners()
	for _, l := range listeners {
		l.OnWebSocketClosed(ws)
	}
}

func (ws *WebSocket) addListener(l WebSocketListener) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	for _, listener := range ws.listeners {
		if l == listener {
			return
		}
	}
	ws.listeners = append(ws.listeners, l)

	select {
	case <-ws.closed:
		l.OnWebSocketClosed(ws)
	default:
		l.OnWebSocketOpened(ws)
	}
}

func (ws *WebSocket) removeListener(l WebSocketListener) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	filtered := ws.listeners[0:0]
	for _, listener := range ws.listeners {
		if listener != l {
			filtered = append(filtered, listener)
		}
	}
	ws.listeners = filtered
}

func (ws *WebSocket) copyListeners() []WebSocketListener {
	ws.mu.Lock()
	if len(ws.listeners) == 0 {
		ws.mu.Unlock()
		return nil
	}

	listeners := make([]WebSocketListener, len(ws.listeners))
	copy(listeners, ws.listeners)
	ws.mu.Unlock()
	return listeners
}
