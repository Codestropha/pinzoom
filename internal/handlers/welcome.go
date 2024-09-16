package handlers

import (
	"fmt"
	"html/template"
	"pinzoom/pkg/hub"
)

func Welcome(ctx *hub.Ctx) error {
	ctx.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.ParseFiles(
		"./views/welcome.html",
		"./views/layouts/main.html",
		"./views/partials/head.html",
		"./views/partials/header.html",
	)
	if err != nil {
		return fmt.Errorf("error while parsing template files, err=%v", err)
	}
	if err = tmpl.ExecuteTemplate(ctx.Response, "main", nil); err != nil {
		return fmt.Errorf("error while executing template files, err=%v", err)
	}
	return nil
}
