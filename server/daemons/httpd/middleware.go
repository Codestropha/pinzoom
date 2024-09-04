package httpd

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"pinzoom/library/httpd"
	"pinzoom/schema"
	"time"
)

func (s *Server) contextMiddleware(ctx0 context.Context, req *http.Request, resp *http.Response, next http.Handler) error {
	// TODO
	return nil
}

func (s *Server) loggerMiddleware(ctx context.Context, req *httpd.Req, resp *httpd.Resp, next httpd.Handler) error {
	t0 := time.Now().Truncate(time.Millisecond)
	err := next(ctx, req, resp)
	t1 := time.Now().Truncate(time.Millisecond)

	logrus.WithContext(ctx).Debugf("%v %v %d %v %db", req.Method, req.RequestURI, resp.Status, t1.Sub(t0), resp.TotalBytes)
	return err
}

func (s *Server) errorMiddleware(ctx context.Context, req *httpd.Req, resp *httpd.Resp, next httpd.Handler) error {
	defer func() {
		e := recover()
		if e == nil {
			return
		}

		logrus.WithContext(ctx).Warn(ctx, "Panic %v %v %v", req.Method, req.RequestURI, e)
		resp.ErrorInternal()
	}()

	err := next(ctx, req, resp)
	if err == nil {
		return nil
	}

	appErr, ok := schema.NewErrorFromErr(err)
	if !ok {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		text := fmt.Sprintf("Internal server error, err=%v", err)
		logrus.WithContext(ctx).Error(text)
		return resp.ErrorInternal()
	}

	text := fmt.Sprintf("%v", appErr)
	logrus.WithContext(ctx).Error(text)
	return resp.Error(appErr.Text, appErr.Status())
}

func (s *Server) corsMiddleware(ctx context.Context, req *httpd.Req, resp *httpd.Resp, next httpd.Handler) error {
	origin := req.Header.Get("Origin")
	if len(origin) > 0 {
		resp.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		resp.Header().Set("Access-Control-Allow-Origin", "*")
	}

	resp.Header().Set("Access-Control-Allow-Credentials", "true")
	resp.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
	resp.Header().Set("Access-Control-Allow-Headers",
		"Accept, "+
			"Accept-Encoding, "+
			"Accept-Language, "+
			"Access-Control-Request-Headers, "+
			"Access-Control-Request-Method, "+
			"Authorization, "+
			"Cache-Control, "+
			"Connection, "+
			"Content-Type, "+
			"Cookie, "+
			"Host, "+
			"Origin, "+
			"Pragma, "+
			"Referer, "+
			"X-Requested-With, "+
			"User-Agent")

	if req.Method == "OPTIONS" {
		resp.WriteHeader(http.StatusOK)
		return nil
	}

	return next(ctx, req, resp)
}
