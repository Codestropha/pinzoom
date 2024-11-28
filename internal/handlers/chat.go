package handlers

import (
	"fmt"
	"html/template"
	"log"
	"pinzoom/pkg/chat"
	"pinzoom/pkg/hub"
	"pinzoom/pkg/webrtc"
)

func RoomChat(ctx *hub.Ctx) error {
	tmpl, err := template.ParseFiles("views/chat.html", "views/layouts/main.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		return fmt.Errorf("error while parsing template, err=%v", err)
	}
	if err = tmpl.ExecuteTemplate(ctx.Response, "main", nil); err != nil {
		log.Printf("Error executing template: %v", err)
		return fmt.Errorf("error while executing template, err=%v", err)
	}
	return nil
}

func RoomChatWebsocket(ctx *hub.Ctx) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		log.Println("No uuid parameter provided")
		return fmt.Errorf("missing uuid parameter")
	}

	webrtc.RoomsLock.Lock()
	room := webrtc.Rooms[uuid]
	webrtc.RoomsLock.Unlock()
	if room == nil {
		return nil
	}
	if room.Hub == nil {
		return nil
	}
	chat.PeerChatConn(ctx.WebSocket, room.Hub)
	return nil
}

func StreamChatWebsocket(ctx *hub.Ctx) error {
	suuid := ctx.Param("suuid")
	if suuid == "" {
		log.Println("No suuid parameter provided")
		return fmt.Errorf("missing suuid parameter")
	}

	webrtc.RoomsLock.Lock()
	if stream, ok := webrtc.Streams[suuid]; ok {
		webrtc.RoomsLock.Unlock()
		if stream.Hub == nil {
			hub := chat.NewHub()
			stream.Hub = hub
			go hub.Run()
		}
		chat.PeerChatConn(ctx.WebSocket, stream.Hub)
		return nil
	}
	webrtc.RoomsLock.Unlock()
	return nil
}
