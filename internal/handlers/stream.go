package handlers

import (
	"fmt"
	"html/template"
	"os"
	"pinzoom/pkg/hub"
	w "pinzoom/pkg/webrtc"
	ws "pinzoom/pkg/websocket"
	"time"
)

func Stream(ctx *hub.Ctx) error {
	suuid := ctx.Param("suuid")
	if suuid == "" {
		return fmt.Errorf("there no suuid param")
	}
	wsProto := "wss"
	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		wsProto = "wss"
	} else {
		wsProto = "ws"
	}

	w.RoomsLock.Lock()
	_, streamExists := w.Streams[suuid]
	w.RoomsLock.Unlock()

	tmpl, err := template.ParseFiles("views/stream.html", "views/layouts/main.html")
	if err != nil {
		return fmt.Errorf("error while parsing template, err=%v", err)
	}
	data := map[string]interface{}{
		"Type": "stream",
	}

	if streamExists {
		data["StreamWebsocketAddr"] = fmt.Sprintf("%s://%s/stream/%s/websocket", wsProto, ctx.Host(), suuid)
		data["ChatWebsocketAddr"] = fmt.Sprintf("%s://%s/stream/%s/chat/websocket", wsProto, ctx.Host(), suuid)
		data["ViewerWebsocketAddr"] = fmt.Sprintf("%s://%s/stream/%s/viewer/websocket", wsProto, ctx.Host(), suuid)
	} else {
		data["NoStream"] = "true"
		data["Leave"] = "true"
	}
	if err = tmpl.ExecuteTemplate(ctx.Response, "main", data); err != nil {
		return fmt.Errorf("error while executing template, err=%v", err)
	}
	return nil
}

func StreamWebsocket(ctx *hub.Ctx) error {
	suuid := ctx.Param("suuid")
	if suuid == "" {
		return fmt.Errorf("there no suuid param")
	}

	w.RoomsLock.Lock()
	if stream, ok := w.Streams[suuid]; ok {
		w.RoomsLock.Unlock()
		return w.StreamConn(ctx.WebSocket, stream.Peers)
	}
	w.RoomsLock.Unlock()
	return nil
}

func StreamViewerWebsocket(ctx *hub.Ctx) error {
	suuid := ctx.Param("suuid")
	if suuid == "" {
		return fmt.Errorf("there no suuid param")
	}

	w.RoomsLock.Lock()
	if stream, ok := w.Streams[suuid]; ok {
		w.RoomsLock.Unlock()
		viewerConn(ctx.WebSocket, stream.Peers)
		return nil
	}
	w.RoomsLock.Unlock()
	return nil
}

func viewerConn(c *ws.WebSocket, p *w.Peers) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer c.Close()

	for {
		select {
		case <-ticker.C:
			writer, err := c.NextWriter(ws.TextMessage)
			if err != nil {
				return
			}
			writer.Write([]byte(fmt.Sprintf("%d", len(p.Connections))))
		}
	}
}
