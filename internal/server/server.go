package server

import (
	"context"
	"flag"
	"github.com/sirupsen/logrus"
	"os"
	"pinzoom/internal/handlers"
	"pinzoom/pkg/router"
	"pinzoom/pkg/webrtc"
	"time"
)

var (
	addr = flag.String("addr", ":"+os.Getenv("PORT"), "")
	cert = flag.String("cert", "", "")
	key  = flag.String("key", "", "")
)

func Run(ctx context.Context) error {
	flag.Parse()

	if *addr == ":" {
		*addr = ":8080"
	}

	app := router.NewRouter()
	app.Use(router.CORSMiddleware)
	app.Use(router.ErrorMiddleware)

	app.Get("/", handlers.Welcome)
	app.Get("/room/create", handlers.RoomCreate)
	app.Get("/room/:uuid", handlers.Room)
	app.Get("/room/:uuid/websocket", handlers.RoomWebsocket)
	app.Get("/room/:uuid/chat", handlers.RoomChat)
	app.Get("/room/:uuid/chat/websocket", handlers.RoomChatWebsocket)
	app.Get("/room/:uuid/viewer/websocket", handlers.RoomViewerWebsocket)
	app.Get("/stream/:suuid", handlers.Stream)
	app.Get("/stream/:suuid/websocket", handlers.StreamWebsocket)
	app.Get("/stream/:suuid/chat/websocket", handlers.StreamChatWebsocket)
	app.Get("/stream/:suuid/viewer/websocket", handlers.StreamViewerWebsocket)
	app.Static("./assets")

	webrtc.Rooms = make(map[string]*webrtc.Room)
	webrtc.Streams = make(map[string]*webrtc.Room)
	go dispatchKeyFrames()

	go func() {
		if *cert != "" {
			if err := app.ListenAndServeTLS(*addr, *cert, *key); err != nil {
				logrus.Error(err)
			}
		} else if err := app.ListenAndServe(*addr); err != nil {
			logrus.Error(err)
		}
	}()

	<-ctx.Done()

	return nil
}

func dispatchKeyFrames() {
	for range time.NewTicker(time.Second * 3).C {
		for _, room := range webrtc.Rooms {
			room.Peers.DispatchKeyFrame()
		}
	}
}
