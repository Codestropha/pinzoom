package router

import (
	"github.com/sirupsen/logrus"
	"net/http"
	"pinzoom/pkg/hub"
)

func CORSMiddleware(next HandlerFunc) HandlerFunc {
	return func(ctx *hub.Ctx) error {
		ctx.Response.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		ctx.Response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		ctx.Response.Header().Set("Access-Control-Allow-Credentials", "true")

		if ctx.Request.Method == "OPTIONS" {
			ctx.Response.WriteHeader(http.StatusNoContent)
			return nil
		}

		return next(ctx)
	}
}

func ErrorMiddleware(next HandlerFunc) HandlerFunc {
	return func(ctx *hub.Ctx) error {
		defer func() {
			if err := recover(); err != nil {
				logrus.Printf("Internal server error: %v", err)
				http.Error(ctx.Response, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		return next(ctx)
	}
}
