package router

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type Response struct {
	conn        net.Conn
	header      http.Header
	status      int
	headersSent bool
}

func (w *Response) Header() http.Header {
	return w.header
}

func (w *Response) WriteHeader(statusCode int) {
	if !w.headersSent {
		w.status = statusCode
		w.Write(nil) // Trigger sending headers with status
	}
}

func (w *Response) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if !w.headersSent {
		statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", w.status, http.StatusText(w.status))
		_, err := w.conn.Write([]byte(statusLine))
		if err != nil {
			return 0, err
		}

		for key, values := range w.header {
			for _, value := range values {
				_, err := w.conn.Write([]byte(fmt.Sprintf("%s: %s\r\n", key, value)))
				if err != nil {
					return 0, err
				}
			}
		}

		_, err = w.conn.Write([]byte("\r\n"))
		if err != nil {
			return 0, err
		}
		w.headersSent = true
	}

	return w.conn.Write(data)
}

func NewResponseWriter(conn net.Conn) *Response {
	return &Response{
		conn:        conn,
		header:      make(http.Header),
		status:      0,
		headersSent: false,
	}
}

func (w *Response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.conn == nil {
		return nil, nil, fmt.Errorf("connection is not available")
	}
	rw := bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn))
	return w.conn, rw, nil
}
