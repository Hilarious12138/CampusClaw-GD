package main

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPI []byte

func (a *App) openAPIHandler(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	_, e := w.Write(openAPI)
	return e
}
