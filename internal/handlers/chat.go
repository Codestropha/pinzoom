package handlers

import (
	"fmt"
	"html/template"
	"pinzoom/pkg/chat"
	"pinzoom/pkg/hub"
	"pinzoom/pkg/webrtc"
)

func RoomChat(ctx *hub.Ctx) error {
	tmpl, err := template.ParseFiles("views/chat.html", "views/layouts/main.html")
	if err != nil {
		return fmt.Errorf("error while parsing template, err=%v", err)
	}
	if err = tmpl.ExecuteTemplate(ctx.Response, "main", nil); err != nil {
		return fmt.Errorf("error while executing template, err=%v", err)
	}
	return nil
}

func RoomChatWebsocket(ctx *hub.Ctx) error {
	uuidFromParam := ctx.Param("uuid")
	if uuidFromParam == "" {
		return fmt.Errorf("there no uuid param")
	}

	webrtc.RoomsLock.Lock()
	room := webrtc.Rooms[uuidFromParam]
	webrtc.RoomsLock.Unlock()
	if room == nil {
		return nil
	}
	if room.Hub == nil {
		return nil
	}

	chat.PeerChatConn(ctx, room.Hub)
	return nil
}

func StreamChatWebsocket(ctx *hub.Ctx) error {
	suuid := ctx.Param("suuid")
	if suuid == "" {
		return fmt.Errorf("there no suuid param")
	}

	webrtc.RoomsLock.Lock()
	if stream, ok := webrtc.Streams[suuid]; ok {
		webrtc.RoomsLock.Unlock()
		if stream.Hub == nil {
			hub := chat.NewHub()
			stream.Hub = hub
			go hub.Run()
		}
		chat.PeerChatConn(ctx, stream.Hub)
		return nil
	}
	webrtc.RoomsLock.Unlock()
	return nil
}
