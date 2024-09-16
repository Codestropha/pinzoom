package handlers

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"os"
	"pinzoom/pkg/chat"
	"pinzoom/pkg/hub"
	w "pinzoom/pkg/webrtc"
	ws "pinzoom/pkg/websocket"
	"time"

	"crypto/sha256"
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
		return fmt.Errorf("there no uuid param")
	}
	logrus.Println(uuidFromParam)

	wsProto := "ws"
	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		wsProto = "wss"
	}

	uuidFromParam, suuid, _ := createOrGetRoom(uuidFromParam)
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
		return fmt.Errorf("error while parsing template, err=%v", err)
	}

	if err = tmpl.ExecuteTemplate(ctx.Response, "main", data); err != nil {
		return fmt.Errorf("error while executing template, err=%v", err)
	}
	return nil
}

func RoomWebsocket(ctx *hub.Ctx) error {
	if ctx.WebSocket == nil {
		return fmt.Errorf("ws connection not found")
	}
	uuidFromParam := ctx.Param("uuid")
	if uuidFromParam == "" {
		return fmt.Errorf("there no uuid param")
	}

	_, _, room := createOrGetRoom(uuidFromParam)
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
	p := &w.Peers{}
	p.TrackLocals = make(map[string]*webrtc.TrackLocalStaticRTP)
	room := &w.Room{
		Peers: p,
		Hub:   hub,
	}

	w.Rooms[uuid] = room
	w.Streams[suuid] = room

	go hub.Run()
	return uuid, suuid, room
}

func RoomViewerWebsocket(ctx *hub.Ctx) error {
	uuidFromParam := ctx.Param("uuid")
	if uuidFromParam == "" {
		return fmt.Errorf("there no uuid param")
	}

	w.RoomsLock.Lock()
	if peer, ok := w.Rooms[uuidFromParam]; ok {
		w.RoomsLock.Unlock()
		roomViewerConn(ctx.WebSocket, peer.Peers)
		return nil
	}
	w.RoomsLock.Unlock()
	return nil
}

func roomViewerConn(c *ws.WebSocket, p *w.Peers) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer c.Close()

	for {
		select {
		case <-ticker.C:
			w, err := c.NextWriter(ws.TextMessage)
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
