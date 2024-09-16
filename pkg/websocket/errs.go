package ws

import "fmt"

type CloseError struct {
	Code int
	Text string
}

func (e *CloseError) Error() string {
	return fmt.Sprintf("close error: code=%d, text=%s", e.Code, e.Text)
}

func NewCloseError(code int, text string) *CloseError {
	return &CloseError{
		Code: code,
		Text: text,
	}
}

type netError struct {
	msg       string
	temporary bool
	timeout   bool
}

func (e *netError) Error() string   { return e.msg }
func (e *netError) Temporary() bool { return e.temporary }
func (e *netError) Timeout() bool   { return e.timeout }
