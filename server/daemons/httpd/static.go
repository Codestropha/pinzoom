package httpd

import (
	"context"
	"html/template"
	"net/http"
	"pinzoom/library/httpd"
)

func (s *Server) static(r *httpd.Router) {
	r.Static("/static/*", "./static/")
	r.GET("/", s.staticAppRoot)
	r.GET("/*", s.staticAppRoot)
}

func (s *Server) staticAppRoot(ctx context.Context, req *httpd.Req, resp *httpd.Resp) error {
	t, err := template.ParseFiles("./static/index.html")
	if err != nil {
		return err
	}

	resp.WriteHeader(http.StatusOK)
	return t.Execute(resp, "")
}
