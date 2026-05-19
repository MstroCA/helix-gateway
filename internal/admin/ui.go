package admin

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui
var uiFS embed.FS

func serveUI(mux *http.ServeMux) {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return
	}
	mux.Handle("/admin/ui/", http.StripPrefix("/admin/ui/", http.FileServer(http.FS(sub))))
}
