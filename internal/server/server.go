package server

import (
	"context"
	"flag"
	"os"
	"pinzoom/internal/handlers"
	"pinzoom/pkg/router"
	"pinzoom/pkg/webrtc"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	cert = flag.String("cert", "", "")
	key  = flag.String("key", "", "")
)

func Run(ctx context.Context) error {
	if err := os.Setenv("ENVIRONMENT", "PRODUCTION"); err != nil {
		return err
	}
	flag.Parse()

	app := router.NewRouter()
	app.Use(router.CORSMiddleware)
	app.Use(router.ErrorMiddleware)

	app.Get("/", handlers.Welcome)
	app.Get("/room/create", handlers.RoomCreate)
	app.Get("/room/:uuid", handlers.Room)
	app.Get("/room/:uuid/websocket", router.WebSocketHandler(router.WebSocketHandler{
		Handler: handlers.RoomWebsocket,
	}).ToHandlerFunc())
	app.Get("/room/:uuid/chat", handlers.RoomChat)
	app.Get("/room/:uuid/chat/websocket", router.WebSocketHandler(router.WebSocketHandler{
		Handler:          handlers.RoomChatWebsocket,
		HandshakeTimeout: 10 * time.Second,
	}).ToHandlerFunc())
	app.Get("/room/:uuid/viewer/websocket", router.WebSocketHandler(router.WebSocketHandler{
		Handler: handlers.RoomViewerWebsocket,
	}).ToHandlerFunc())
	app.Get("/stream/:suuid", router.WebSocketHandler(router.WebSocketHandler{
		Handler: handlers.Stream,
	}).ToHandlerFunc())
	app.Get("/stream/:suuid/websocket", router.WebSocketHandler(router.WebSocketHandler{
		Handler:          handlers.StreamWebsocket,
		HandshakeTimeout: 10 * time.Second,
	}).ToHandlerFunc())
	app.Get("/stream/:suuid/chat/websocket", router.WebSocketHandler(router.WebSocketHandler{
		Handler: handlers.StreamChatWebsocket,
	}).ToHandlerFunc())
	app.Get("/stream/:suuid/viewer/websocket", router.WebSocketHandler(router.WebSocketHandler{
		Handler: handlers.StreamViewerWebsocket,
	}).ToHandlerFunc())
	app.Static("./assets")

	webrtc.Rooms = make(map[string]*webrtc.Room)
	webrtc.Streams = make(map[string]*webrtc.Room)
	go dispatchKeyFrames()

	go func() {
		if err := app.ListenAndServe(":8080"); err != nil {
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
