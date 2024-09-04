package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"pinzoom/config"
	"pinzoom/ctx"
	"pinzoom/daemons/httpd"
	"pinzoom/library/async"
)

func New(config *config.Config) *Server {
	return &Server{
		services: []async.Service{
			httpd.NewServer(config),
		},
	}
}

type Server struct {
	services []async.Service
}

func (s *Server) Migrate() error {
	_ = ctx.Background()

	// some migration process...

	return nil
}

func (s *Server) Run(parent context.Context, config *config.Config) error {
	c := ctx.New(parent)

	// db init...

	logrus.Warn("Starting...")
	logrus.Warn("Started...")
	defer logrus.Warn("Stopped")
	services := async.Group(s.services...)
	defer services.StopAndWait()
	defer logrus.Warn("Stopping...")

	select {
	case <-services.Start():
	case <-c.Done():
		return nil
	}
	if err := services.StartError(); err != nil {
		return err
	}
	<-c.Done()

	return nil
}
