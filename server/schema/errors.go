package schema

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrNotFound = &Error{ErrorCodeNotFound, "Not found"}
)

type Error struct {
	Code ErrorCode
	Text string
}

func (e *Error) String() string {
	return fmt.Sprintf("%v: %v", e.Code, e.Text)
}

func (e *Error) Error() string {
	return e.Text
}

func (e *Error) Status() int {
	status, ok := errorStatus[e.Code]
	if !ok {
		return http.StatusInternalServerError
	}
	return status
}

func NewError(code ErrorCode, format string, args ...interface{}) *Error {
	return &Error{
		Code: code,
		Text: fmt.Sprintf(format, args...),
	}
}

func NewInvalidError(format string, args ...interface{}) *Error {
	return NewError(ErrorCodeInvalid, format, args...)
}

func NewNotFoundError(format string, args ...interface{}) *Error {
	return NewError(ErrorCodeNotFound, format, args...)
}

func NewErrorFromErr(err error) (*Error, bool) {
	var appErr *Error
	if ok := errors.As(err, &appErr); ok {
		return appErr, true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound, true
	}
	return nil, false
}

type ErrorCode string

const (
	ErrorCodeNotFound     ErrorCode = "not_found"
	ErrorCodeForbidden    ErrorCode = "forbidden"
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	ErrorCodeInvalid      ErrorCode = "invalid"
)

var errorStatus = map[ErrorCode]int{
	ErrorCodeNotFound:     http.StatusNotFound,
	ErrorCodeForbidden:    http.StatusForbidden,
	ErrorCodeUnauthorized: http.StatusUnauthorized,
	ErrorCodeInvalid:      http.StatusUnprocessableEntity,
}
