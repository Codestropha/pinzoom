package router

import (
	"bufio"
	"crypto/tls"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"pinzoom/pkg/hub"
	"regexp"
	"strings"
)

type HandlerFunc func(*hub.Ctx) error

type Router struct {
	routes     []Route
	assetsDir  string
	middleware []func(HandlerFunc) HandlerFunc
}

type Route struct {
	method  string
	regex   *regexp.Regexp
	handler HandlerFunc
}

func NewRouter() *Router {
	return &Router{
		routes:     make([]Route, 0),
		middleware: []func(HandlerFunc) HandlerFunc{},
	}
}

func (r *Router) Use(mw func(HandlerFunc) HandlerFunc) {
	r.middleware = append(r.middleware, mw)
}

func (r *Router) Get(path string, handler HandlerFunc) {
	r.Add(http.MethodGet, path, handler)
}

func (r *Router) Add(method, path string, handler HandlerFunc) {
	regexPath := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	regexPattern := regexPath.ReplaceAllString(path, `(?P<$1>[^/]+)`)
	regexPattern = "^" + regexPattern + "$"

	regex := regexp.MustCompile(regexPattern)
	r.routes = append(r.routes, Route{
		method:  method,
		regex:   regex,
		handler: handler,
	})
}

func (r *Router) Static(dir string) {
	r.assetsDir = dir
}

func (r *Router) Serve(ctx *hub.Ctx) error {
	for _, route := range r.routes {
		if route.method == ctx.Request.Method && route.regex.MatchString(ctx.Request.URL.Path) {
			matches := route.regex.FindStringSubmatch(ctx.Request.URL.Path)
			params := make(map[string]string)
			for i, name := range route.regex.SubexpNames() {
				if i > 0 && name != "" {
					params[name] = matches[i]
				}
			}
			ctx.SetParams(params)

			if strings.EqualFold(ctx.Request.Header.Get("Upgrade"), "websocket") {
				if err := ctx.Upgrade(); err != nil {
					return err
				}
				return route.handler(ctx)
			}

			for i := len(r.middleware) - 1; i >= 0; i-- {
				route.handler = r.middleware[i](route.handler)
			}

			return route.handler(ctx)
		}
	}

	if r.assetsDir != "" {
		filePath := filepath.Join(r.assetsDir, strings.TrimPrefix(ctx.Request.URL.Path, "/"))
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			http.ServeFile(ctx.Response, ctx.Request, filePath)
			return nil
		}
	}

	http.NotFound(ctx.Response, ctx.Request)
	return nil
}

func (r *Router) ListenAndServe(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.Error("Error starting TCP listener:", err)
	}
	defer listener.Close()

	logrus.Printf("Server is running on %s\n", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.Println("Error accepting connection:", err)
			continue
		}

		go func(c net.Conn) {
			ctx := hub.NewContext(nil, nil, nil, nil, c)
			defer func() {
				if ctx.Proto() != hub.ProtoWS {
					c.Close()
				}
			}()

			req, err := http.ReadRequest(bufio.NewReader(c))
			if err != nil {
				logrus.Println("Error reading request:", err)
				return
			}
			logrus.Println("Request", req.Method, req.URL, req.Body)

			respWriter := NewResponseWriter(c)
			ctx.Request = req
			ctx.Response = respWriter

			if err = r.Serve(ctx); err != nil {
				logrus.Println("Error serving http:", err)
				return
			}
			logrus.Println("Response", req.Method, req.URL, respWriter.status, req.Body)
		}(conn)
	}
}

func (r *Router) ListenAndServeTLS(addr, certFile, keyFile string) error {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		logrus.Error("Error starting TLS listener:", err)
	}
	defer listener.Close()

	logrus.Printf("Server is running on %s with TLS\n", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.Println("Error accepting connection:", err)
			continue
		}

		go func(c net.Conn) {
			ctx := hub.NewContext(nil, nil, nil, nil, c)
			defer func() {
				if ctx.Proto() != hub.ProtoWS {
					c.Close()
				}
			}()
			req, err := http.ReadRequest(bufio.NewReader(c))
			if err != nil {
				logrus.Println("Error reading request:", err)
				return
			}

			respWriter := NewResponseWriter(c)
			ctx.Request = req
			ctx.Response = respWriter

			if err = r.Serve(ctx); err != nil {
				logrus.Println("Error serving http:", err)
				return
			}
		}(conn)
	}
}
