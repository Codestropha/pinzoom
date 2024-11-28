package router

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"pinzoom/pkg/hub"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
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

			// Apply middleware in registered order
			for _, mw := range r.middleware {
				route.handler = mw(route.handler)
			}

			if err := route.handler(ctx); err != nil {
				logrus.Error("Error handling request:", err)
			}
			return nil
		}
	}

	// Serve static files securely
	if r.assetsDir != "" {
		filePath := filepath.Join(r.assetsDir, filepath.Clean(strings.TrimPrefix(ctx.Request.URL.Path, "/")))
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

		go r.handleConnection(conn)
	}
}

func (r *Router) handleConnection(c net.Conn) {
	defer c.Close()
	ctx := hub.NewContext(nil, nil, nil, nil, c)
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		logrus.Println("Error reading fd:", err)
		return
	}
	reader := bytes.NewBuffer(buf[:n])
	req, err := http.ReadRequest(bufio.NewReader(reader))
	if err != nil {
		logrus.Println("Error reading request:", err)
		return
	}
	respWriter := NewResponseWriter(c)
	ctx.Request = req
	ctx.Response = respWriter

	if err := r.Serve(ctx); err != nil {
		logrus.Println("Error serving request:", err)
	}
}
