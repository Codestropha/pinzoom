package handlers

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"pinzoom/pkg/hub"
	w "pinzoom/pkg/webrtc"
	"time"

	"github.com/gorilla/websocket"
)

func Stream(ctx *hub.Ctx) error {
	suuid := ctx.Param("suuid")
	if suuid == "" {
		log.Println("No suuid parameter provided")
		return fmt.Errorf("missing suuid parameter")
	}

	wsProto := "ws"
	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		wsProto = "wss"
	}

	w.RoomsLock.Lock()
	_, streamExists := w.Streams[suuid]
	w.RoomsLock.Unlock()

	tmpl, err := template.ParseFiles("views/stream.html", "views/layouts/main.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		return fmt.Errorf("template parsing error: %v", err)
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
		log.Printf("Error executing template: %v", err)
		return fmt.Errorf("template execution error: %v", err)
	}
	return nil
}

func StreamWebsocket(ctx *hub.Ctx) error {
	if ctx.WebSocket == nil {
		log.Println("WebSocket connection not found")
		return fmt.Errorf("WebSocket connection not found")
	}

	suuid := ctx.Param("suuid")
	if suuid == "" {
		log.Println("No suuid parameter provided")
		return fmt.Errorf("missing suuid parameter")
	}

	w.RoomsLock.Lock()
	if stream, ok := w.Streams[suuid]; ok {
		w.RoomsLock.Unlock()
		w.StreamConn(ctx.WebSocket, stream.Peers)
		return nil
	}
	w.RoomsLock.Unlock()
	return nil
}

func StreamViewerWebsocket(ctx *hub.Ctx) error {
	if ctx.WebSocket == nil {
		log.Println("WebSocket connection not found")
		return fmt.Errorf("WebSocket connection not found")
	}

	suuid := ctx.Param("suuid")
	if suuid == "" {
		log.Println("No suuid parameter provided")
		return fmt.Errorf("missing suuid parameter")
	}

	w.RoomsLock.Lock()
	stream, ok := w.Streams[suuid]
	w.RoomsLock.Unlock()
	if !ok {
		log.Printf("Stream with suuid %s not found", suuid)
		return fmt.Errorf("stream with suuid %s not found", suuid)
	}

	log.Printf("Starting viewer connection for stream %s", suuid)
	viewerConn(ctx.WebSocket, stream.Peers)
	return nil
}

func viewerConn(c *websocket.Conn, p *w.Peers) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer c.Close()

	for {
		select {
		case <-ticker.C:
			writer, err := c.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Error creating WebSocket writer: %v", err)
				return
			}
			if _, err := writer.Write([]byte(fmt.Sprintf("%d", len(p.Connections)))); err != nil {
				log.Printf("Error writing to WebSocket: %v", err)
				writer.Close()
				return
			}
			writer.Close() // Ensure the writer is closed after each use
		}
	}
}
