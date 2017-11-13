package main

import (
	json "encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/client/r0"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
)

const UnkErrJson = `{"code":"M_UNKNOWN","message":"Unexpected error processing response"}`

type Handler struct {
	h func(http.ResponseWriter, *http.Request, storage.Database, config.MediaRepoConfig) interface{}
	opts HandlerOpts
}

type HandlerOpts struct {
	db storage.Database
	config config.MediaRepoConfig
}

type EmptyResponse struct {}

func main() {
	rtr := mux.NewRouter()

	c, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}

	db, err := storage.OpenDatabase(c.Database.Postgres)
	if err != nil {
		panic(err)
	}

	hOpts := HandlerOpts{*db, c}

	uploadHandler := Handler{r0.UploadMedia, hOpts}
	downloadHandler := Handler{r0.DownloadMedia, hOpts}
	thumbnailHandler := Handler{r0.ThumbnailMedia, hOpts}

	rtr.Handle("/_matrix/client/r0/media/upload", uploadHandler).Methods("POST")
	rtr.Handle("/_matrix/client/r0/media/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}", downloadHandler).Methods("GET")
	rtr.Handle("/_matrix/client/r0/media/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}/{filename:[a-zA-Z0-9._-]+}", downloadHandler).Methods("GET")
	rtr.Handle("/_matrix/client/r0/media/thumbnail/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}", thumbnailHandler).Methods("GET")

	http.Handle("/", rtr)
	http.ListenAndServe(":8000", nil)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res := h.h(w, r, h.opts.db, h.opts.config)
	if res == nil {
		res = &EmptyResponse{}
	}

	b, err := json.Marshal(res)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, UnkErrJson, http.StatusInternalServerError)
		return
	}
	jsonStr := string(b)

	switch result := res.(type) {
	case *client.ErrorResponse:
		switch result.InternalCode {
		case "M_NOT_FOUND":
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, jsonStr, http.StatusNotFound)
			break
		case "M_MEDIA_TOO_LARGE":
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, jsonStr, http.StatusRequestEntityTooLarge)
			break
		//case "M_UNKNOWN":
		default:
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, jsonStr, http.StatusInternalServerError)
			break
		}
		break
	case *r0.DownloadMediaResponse:
		w.Header().Set("Content-Type", result.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename=\"" + result.Filename +"\"")
		w.Header().Set("Content-Length", fmt.Sprint(result.SizeBytes))
		f, err := os.Open(result.Location)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, UnkErrJson, http.StatusInternalServerError)
			break
		}
		defer f.Close()
		io.Copy(w, f)
		break
	default:
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, jsonStr)
		break
	}
}