package httpd

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Req struct {
	*http.Request
	Router *Router
	Params Params
}

func newReq(r *Router, r0 *http.Request, params Params) *Req {
	return &Req{
		Router:  r,
		Request: r0,
		Params:  params,
	}
}

func (r *Req) IsHTTPS() bool {
	// Check TLS connection.
	if r.TLS != nil {
		return true
	}

	// Check proxy forwarded protocol
	proto := r.Header.Get("X-Forwarded-Proto")
	proto = strings.ToLower(proto)
	if strings.HasPrefix(proto, "https") {
		return true
	}

	// Check proxy forwarded host
	host := r.Header.Get("X-Forwarded-Host")
	host = strings.ToLower(host)
	if strings.HasPrefix(host, "https") {
		return true
	}

	return false
}

// Params

func (r *Req) Int(param string) int {
	return r.Params.Int(param)
}

func (r *Req) Int32(param string) int32 {
	return r.Params.Int32(param)
}

func (r *Req) Int64(param string) int64 {
	return r.Params.Int64(param)
}

func (r *Req) Param(param string) string {
	return r.Params[param]
}

// Form

func (r *Req) FormInt(key string) int {
	v := r.FormValue(key)
	i, _ := strconv.ParseInt(v, 10, 64)
	return int(i)
}

func (r *Req) FormInt32(key string) int32 {
	v := r.FormValue(key)
	i, _ := strconv.ParseInt(v, 10, 32)
	return int32(i)
}

func (r *Req) FormInt64(key string) int64 {
	v := r.FormValue(key)
	i, _ := strconv.ParseInt(v, 10, 64)
	return i
}

// JSON

// DecodeJSON decodes a JSON body into a given destination.
func (r *Req) DecodeJSON(dst interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(&dst); err != nil {
		return NewBadRequestError(err.Error())
	}
	return nil
}

// WebSocket

func (r *Req) WebSocket(ctx context.Context, resp *Resp) (*WebSocket, error) {
	ws, err := NewWebSocket(ctx, resp.ResponseWriter, r.Request)
	if err != nil {
		return nil, NewBadRequestError(err.Error())
	}

	ws.addListener(r.Router)
	return ws, nil
}
