package hub

import (
	"net"
	"net/http"
	ws "pinzoom/pkg/websocket"
)

type Proto int

const (
	ProtoHttp Proto = iota
	ProtoWS
)

type Ctx struct {
	Request   *http.Request
	Response  http.ResponseWriter
	WebSocket *ws.WebSocket

	conn   net.Conn
	params map[string]string
}

func NewContext(
	req *http.Request,
	w http.ResponseWriter,
	params map[string]string,
	ws *ws.WebSocket,
	conn net.Conn,
) *Ctx {
	c := &Ctx{
		Request:   req,
		Response:  w,
		conn:      conn,
		params:    params,
		WebSocket: ws,
	}
	return c
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

func (c *Ctx) Upgrade() error {
	webSocket, err := ws.NewWebSocket(c.conn, c.Request, c.Response)
	if err != nil {
		return err
	}
	c.WebSocket = webSocket
	return nil
}

func (c *Ctx) SetParams(params map[string]string) {
	c.params = params
}

func (c *Ctx) Proto() Proto {
	if c.WebSocket == nil {
		return 0
	}
	return 1
}
