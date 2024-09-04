package ctx

import (
	"context"
)

type Context interface {
	context.Context
}

func New(parent context.Context) Context {
	return parent
}

type ctx struct {
	context.Context
}

func Background() Context {
	return New(context.Background())
}
