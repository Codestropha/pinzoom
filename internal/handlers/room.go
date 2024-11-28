package handlers

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"pinzoom/pkg/chat"
	"pinzoom/pkg/hub"
	w "pinzoom/pkg/webrtc"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
)

func RoomCreate(ctx *hub.Ctx) error {
	roomID := uuid.New().String()
	ctx.Redirect(fmt.Sprintf("/room/%s", roomID))
	return nil
}

func Room(ctx *hub.Ctx) error {
	ctx.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	uuidFromParam := ctx.Param("uuid")
	if uuidFromParam == "" {
		logrus.Error("UUID parameter is missing in Room request")
		return fmt.Errorf("UUID parameter missing")
	}
	logrus.Infof("Room requested with UUID: %s", uuidFromParam)

	wsProto := "ws"
	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		wsProto = "wss"
	}

	uuidFromParam, suuid, room := createOrGetRoom(uuidFromParam)
	if room == nil {
		return fmt.Errorf("failed to create or retrieve room with UUID: %s", uuidFromParam)
	}

	data := struct {
		RoomWebsocketAddr   string
		RoomLink            string
		ChatWebsocketAddr   string
		ViewerWebsocketAddr string
		StreamLink          string
		Type                string
	}{
		RoomWebsocketAddr:   fmt.Sprintf("%s://%s/room/%s/websocket", wsProto, ctx.Host(), uuidFromParam),
		RoomLink:            fmt.Sprintf("%s://%s/room/%s", getProtocol(ctx.Request), ctx.Host(), uuidFromParam),
		ChatWebsocketAddr:   fmt.Sprintf("%s://%s/room/%s/chat/websocket", wsProto, ctx.Host(), uuidFromParam),
		ViewerWebsocketAddr: fmt.Sprintf("%s://%s/room/%s/viewer/websocket", wsProto, ctx.Host(), uuidFromParam),
		StreamLink:          fmt.Sprintf("%s://%s/stream/%s", getProtocol(ctx.Request), ctx.Host(), suuid),
		Type:                "room",
	}

	tmpl, err := template.ParseFiles(
		"views/peer.html",
		"views/layouts/main.html",
		"./views/partials/head.html",
		"./views/partials/header.html",
		"./views/partials/chat.html",
	)
	if err != nil {
		logrus.Errorf("Template parsing error: %v", err)
		return fmt.Errorf("error parsing template: %v", err)
	}

	if err = tmpl.ExecuteTemplate(ctx.Response, "main", data); err != nil {
		logrus.Errorf("Template execution error: %v", err)
		return fmt.Errorf("error executing template: %v", err)
	}
	return nil
}

func RoomWebsocket(ctx *hub.Ctx) error {
	if ctx.WebSocket == nil {
		logrus.Error("WebSocket connection not found in RoomWebsocket")
		return fmt.Errorf("WebSocket connection not found")
	}
	uuidFromParam := ctx.Param("uuid")
	if uuidFromParam == "" {
		logrus.Error("UUID parameter missing in RoomWebsocket")
		return fmt.Errorf("UUID parameter missing")
	}

	_, _, room := createOrGetRoom(uuidFromParam)
	if room == nil {
		logrus.Errorf("Room with UUID %s not found", uuidFromParam)
		return fmt.Errorf("room with UUID %s not found", uuidFromParam)
	}
	return w.RoomConn(ctx, room.Peers)
}

func createOrGetRoom(uuid string) (string, string, *w.Room) {
	w.RoomsLock.Lock()
	defer w.RoomsLock.Unlock()

	h := sha256.New()
	h.Write([]byte(uuid))
	suuid := fmt.Sprintf("%x", h.Sum(nil))

	if room := w.Rooms[uuid]; room != nil {
		if _, ok := w.Streams[suuid]; !ok {
			w.Streams[suuid] = room
		}
		return uuid, suuid, room
	}

	hub := chat.NewHub()
	p := &w.Peers{TrackLocals: make(map[string]*webrtc.TrackLocalStaticRTP)}
	room := &w.Room{
		Peers: p,
		Hub:   hub,
	}

	w.Rooms[uuid] = room
	w.Streams[suuid] = room

	go hub.Run()
	logrus.Infof("Room and stream created with UUID: %s, SUID: %s", uuid, suuid)
	return uuid, suuid, room
}

func RoomViewerWebsocket(ctx *hub.Ctx) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		logrus.Error("UUID parameter missing in RoomViewerWebsocket")
		return fmt.Errorf("UUID parameter missing")
	}

	w.RoomsLock.Lock()
	if peer, ok := w.Rooms[uuid]; ok {
		w.RoomsLock.Unlock()
		roomViewerConn(ctx.WebSocket, peer.Peers)
		return nil
	}
	w.RoomsLock.Unlock()
	return nil
}

func roomViewerConn(c *websocket.Conn, p *w.Peers) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer c.Close()
	for {
		select {
		case <-ticker.C:
			w, err := c.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write([]byte(fmt.Sprintf("%d", len(p.Connections))))
		}
	}
}

func getProtocol(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
