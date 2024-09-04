package httpd

import (
	"bytes"
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

type Handler func(ctx context.Context, req *Req, resp *Resp) error
type Middleware func(ctx context.Context, req *Req, resp *Resp, next Handler) error

type Router struct {
	route      *Route
	websockets map[*WebSocket]struct{}

	mu     sync.Mutex
	close  bool
	closed chan struct{}

	buffers sync.Pool // sync.Pool<bytes.Buffer>
}

func NewRouter() *Router {
	return &Router{
		route:      NewRoute(),
		websockets: make(map[*WebSocket]struct{}),
		closed:     make(chan struct{}, 8),
	}
}

func (r *Router) ALL(p string, h Handler)        { r.route.ALL(p, h) }
func (r *Router) HEAD(p string, h Handler)       { r.route.HEAD(p, h) }
func (r *Router) GET(p string, h Handler)        { r.route.GET(p, h) }
func (r *Router) POST(p string, h Handler)       { r.route.POST(p, h) }
func (r *Router) PUT(p string, h Handler)        { r.route.PUT(p, h) }
func (r *Router) DELETE(p string, h Handler)     { r.route.DELETE(p, h) }
func (r *Router) Static(p string, root http.Dir) { r.route.Static(p, root) }

func (r *Router) Add(p string, child *Route)            { r.route.Add(p, child) }
func (r *Router) Handler(m string, p string, h Handler) { r.route.Handler(m, p, h) }
func (r *Router) Middleware(p string, m Middleware)     { r.route.Middleware(p, m) }

func (r *Router) ServeHTTP(w http.ResponseWriter, httpReq *http.Request) {
	ctx := httpReq.Context()
	defer func() {
		if err := recover(); err != nil {
			logrus.WithContext(ctx).Error(ctx, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	middleware, handler, params, err := r.route.Match(httpReq.Method, httpReq.URL.Path)
	if err != nil {
		r.handleError(ctx, w, httpReq, err)
		return
	}

	req := newReq(r, httpReq, params)
	resp := newResp(r, w)
	if err = execute(ctx, middleware, handler, req, resp); err != nil {
		r.handleError(ctx, w, httpReq, err)
	}
}

func (r *Router) handleError(ctx context.Context, w http.ResponseWriter, httpReq *http.Request, err error) {
	switch err {
	case ErrRouteNotFound:
		http.NotFound(w, httpReq)

	case ErrMethodNotAllowed:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

	default:
		if bad, ok := err.(BadRequestError); ok {
			http.Error(w, bad.Text, http.StatusBadRequest)
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}

		http.Error(w, "Internal server error", http.StatusInternalServerError)
		logrus.WithContext(ctx).WithError(err).Error("Internal server error")
	}
}

func (r *Router) Close() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.close {
		return r.closed
	}

	r.close = true
	go r.closeAndWait()
	return r.closed
}

func (r *Router) Closed() <-chan struct{} {
	return r.closed
}

func (r *Router) closeAndWait() {
	defer close(r.closed)

	for ws := range r.websockets {
		ws.Close()
	}

	for ws := range r.websockets {
		<-ws.Closed()
	}
}

// webSocketListener

func (r *Router) OnWebSocketOpened(ws *WebSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.close {
		ws.Close()
		return
	}

	r.websockets[ws] = struct{}{}
}

func (r *Router) OnWebSocketClosed(ws *WebSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.close {
		return
	}

	delete(r.websockets, ws)
}

func execute(ctx context.Context, middleware []Middleware, handler Handler, req *Req, resp *Resp) error {
	if len(middleware) == 0 {
		return handler(ctx, req, resp)
	}

	m := middleware[0]
	middleware = middleware[1:]
	return m(ctx, req, resp, func(ctx context.Context, req *Req, resp *Resp) error {
		return execute(ctx, middleware, handler, req, resp)
	})
}

// Buffers

func (r *Router) getBuffer() *bytes.Buffer {
	cached := r.buffers.Get()
	if cached != nil {
		return cached.(*bytes.Buffer)
	}

	return &bytes.Buffer{}
}

func (r *Router) releaseBuffer(buf *bytes.Buffer) {
	buf.Reset()
	r.buffers.Put(buf)
}

func PprofHandler(h http.HandlerFunc) Handler {
	return func(ctx context.Context, req *Req, resp *Resp) error {
		h(resp, req.Request)
		return nil
	}
}
