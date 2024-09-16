package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"pinzoom/internal/server"
	"syscall"
	"time"
)

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
	})
	logrus.SetOutput(os.Stdout)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Run(ctx); err != nil {
			logrus.Error(err)
		}
	}()

	<-sigChan
	cancel()

	timeout := 5 * time.Second
	select {
	case <-ctx.Done():
		logrus.Println("Server stopped gracefully.")
	case <-time.After(timeout):
		logrus.Println("Timeout reached. Forcing shutdown.")
	}
}
