package main

import (
	"net/http"
	"github.com/turt2live/matrix-media-repo/client/r0"
	"github.com/gorilla/mux"
)

func main() {
	rtr := mux.NewRouter()

	rtr.HandleFunc("/_matrix/client/r0/media/upload", r0.UploadMedia).Methods("POST")
	rtr.HandleFunc("/_matrix/client/r0/media/download/{server:[a-zA-Z0-9.]+}/{mediaId:[a-zA-Z0-9]+}", r0.DownloadMedia).Methods("GET")
	rtr.HandleFunc("/_matrix/client/r0/media/download/{server:[a-zA-Z0-9.]+}/{mediaId:[a-zA-Z0-9]+}/{filename:[a-zA-Z0-9._-]+}", r0.DownloadMedia).Methods("GET")
	rtr.HandleFunc("/_matrix/client/r0/media/thumbnail/{server:[a-zA-Z0-9.]+}/{mediaId:[a-zA-Z0-9]+}", r0.ThumbnailMedia).Methods("GET")

	http.Handle("/", rtr)
	http.ListenAndServe(":8000", nil)
}