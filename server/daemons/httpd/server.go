package httpd

import (
	"context"
	"github.com/sirupsen/logrus"
	"net/http"
	"pinzoom/config"
	"pinzoom/ctx"
	"pinzoom/library/async"
	"pinzoom/library/errs"
	"pinzoom/library/httpd"
	"time"
)

type Server struct {
	async.Service
	config *config.Config
}

func NewServer(config *config.Config) *Server {
	s := &Server{
		config: config,
	}
	s.Service = async.NewService(s.serveHTTP)
	return s
}

func (s *Server) router() *httpd.Router {
	r := httpd.NewRouter()

	//r.Middleware("/", s.contextMiddleware)
	r.Middleware("/", s.loggerMiddleware)
	r.Middleware("/", s.corsMiddleware)
	r.Middleware("/", s.errorMiddleware)

	//r.Add("/api", s.api.Routes())
	s.static(r)
	return r
}

// Run loop

func (s *Server) serveHTTP(parent context.Context, started chan<- struct{}) error {
	ctx := ctx.New(parent)
	defer logrus.WithContext(ctx).Info("HTTP stopped")

	router := s.router()
	server := &http.Server{
		Addr:    s.config.Http.Listen,
		Handler: router,
	}
	errorChan := make(chan error, 1)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				logrus.WithContext(ctx).Warn("Panic in HTTP, err=%v", e)

				select {
				case errorChan <- errs.Recovered(e):
				default:
				}
			}
		}()
		err := server.ListenAndServe()
		logrus.WithContext(ctx).WithError(err).Debug("HTTP server exited")

		select {
		case errorChan <- err:
		default:
		}
	}()

	select {
	case err := <-errorChan:
		logrus.WithContext(ctx).Errorf("Failed to listen to %s, err=%v", s.config.Http.Listen, err)
		return err
	case <-time.After(time.Second):
		logrus.WithContext(ctx).Infof("HTTP listening to %v", s.config.Http.Listen)
		close(started)
	case <-ctx.Done():
	}
	<-ctx.Done()
	logrus.WithContext(ctx).Info("Stopping HTTP...")

	router.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	server.Shutdown(shutdownCtx)
	<-router.Closed()
	return nil
}
