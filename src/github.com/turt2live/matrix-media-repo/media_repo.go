package main

import (
	json "encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/client/r0"
	"github.com/turt2live/matrix-media-repo/storage"
)

type Handler struct {
	h func(http.ResponseWriter, *http.Request, storage.Database) interface{}
	db storage.Database
}

type EmptyResponse struct {}

func main() {
	rtr := mux.NewRouter()

	db, err := storage.OpenDatabase("")
	if err != nil {
		panic(err)
	}

	rtr.Handle("/_matrix/client/r0/media/upload", Handler{r0.UploadMedia, *db}).Methods("POST")
	rtr.Handle("/_matrix/client/r0/media/download/{server:[a-zA-Z0-9.]+}/{mediaId:[a-zA-Z0-9]+}", Handler{r0.DownloadMedia, *db}).Methods("GET")
	rtr.Handle("/_matrix/client/r0/media/download/{server:[a-zA-Z0-9.]+}/{mediaId:[a-zA-Z0-9]+}/{filename:[a-zA-Z0-9._-]+}", Handler{r0.DownloadMedia, *db}).Methods("GET")
	rtr.Handle("/_matrix/client/r0/media/thumbnail/{server:[a-zA-Z0-9.]+}/{mediaId:[a-zA-Z0-9]+}", Handler{r0.ThumbnailMedia, *db}).Methods("GET")

	http.Handle("/", rtr)
	http.ListenAndServe(":8000", nil)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	res := h.h(w, r, h.db)
	if res == nil {
		res = &EmptyResponse{}
	}

	b, err := json.Marshal(res)
	if err != nil {
		http.Error(w, `{"code":"M_UNKNOWN","message":"Unexpected error processing response"}`, http.StatusInternalServerError)
		return
	}
	jsonStr := string(b)

	switch result := res.(type) {
	case *client.ErrorResponse:
		switch result.Code {
		//case "M_UNKNOWN":
		default:
			http.Error(w, jsonStr, http.StatusInternalServerError)
			break
		}
		break
	default:
		io.WriteString(w, jsonStr)
	}
}