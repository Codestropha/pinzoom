package router

import (
	"net/http"
	"pinzoom/pkg/hub"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type WebSocketHandler struct {
	Handler          func(*hub.Ctx) error
	HandshakeTimeout time.Duration
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h WebSocketHandler) Serve(ctx *hub.Ctx) error {
	// Устанавливаем таймаут для рукопожатия
	upgrader.HandshakeTimeout = h.HandshakeTimeout

	if err := Upgrade(ctx); err != nil {
		logrus.Error("Failed to upgrade to WebSocket:", err)
		return err
	}

	// Запускаем обработчик
	if err := h.Handler(ctx); err != nil {
		logrus.Error("Error handling WebSocket request:", err)
		return err
	}

	return nil
}

func Upgrade(ctx *hub.Ctx) error {
	if ctx.WebSocket != nil {
		logrus.Warn("Connection already upgraded to WebSocket")
		return nil
	}

	ws, err := upgrader.Upgrade(ctx.Response, ctx.Request, nil)
	if err != nil {
		logrus.Error("Failed to upgrade connection to WebSocket:", err)
		return err
	}

	ctx.WebSocket = ws
	logrus.Info("WebSocket connection established")
	return nil
}

func (h WebSocketHandler) ToHandlerFunc() HandlerFunc {
	return func(ctx *hub.Ctx) error {
		return h.Serve(ctx)
	}
}
