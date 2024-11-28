package hub

import (
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

type Proto int

const (
	ProtoHttp Proto = iota
	ProtoWS
)

type Ctx struct {
	Request   *http.Request
	Response  http.ResponseWriter
	WebSocket *websocket.Conn

	conn   net.Conn
	params map[string]string
}

func NewContext(
	req *http.Request,
	w http.ResponseWriter,
	params map[string]string,
	ws *websocket.Conn,
	conn net.Conn,
) *Ctx {
	return &Ctx{
		Request:   req,
		Response:  w,
		conn:      conn,
		params:    params,
		WebSocket: ws,
	}
}

func (c *Ctx) Param(key string) string {
	return c.params[key]
}

func (c *Ctx) Host() string {
	return c.Request.Host
}

func (c *Ctx) Redirect(url string) {
	http.Redirect(c.Response, c.Request, url, http.StatusFound)
}

func (c *Ctx) SetParams(params map[string]string) {
	c.params = params
}

func (c *Ctx) Proto() Proto {
	if c.WebSocket == nil {
		return ProtoHttp
	}
	return ProtoWS
}
