package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/client/r0"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/logging"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/util"
)

const UnkErrJson = `{"code":"M_UNKNOWN","message":"Unexpected error processing response"}`

type requestCounter struct {
	lastId int
}

type Handler struct {
	h    func(http.ResponseWriter, *http.Request, rcontext.RequestInfo) interface{}
	opts HandlerOpts
}

type HandlerOpts struct {
	db         storage.Database
	config     config.MediaRepoConfig
	reqCounter *requestCounter
}

type EmptyResponse struct{}

func main() {
	rtr := mux.NewRouter()

	c, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}

	err = logging.Setup(c.General.LogDirectory)
	if err != nil {
		panic(err)
	}

	log.Info("Starting media repository...")

	db, err := storage.OpenDatabase(c.Database.Postgres)
	if err != nil {
		panic(err)
	}

	counter := requestCounter{}
	hOpts := HandlerOpts{*db, c, &counter}

	uploadHandler := Handler{r0.UploadMedia, hOpts}
	downloadHandler := Handler{r0.DownloadMedia, hOpts}
	thumbnailHandler := Handler{r0.ThumbnailMedia, hOpts}
	previewUrlHandler := Handler{r0.PreviewUrl, hOpts}

	// r0 endpoints
	log.Info("Registering r0 endpoints")
	rtr.Handle("/_matrix/media/r0/upload", uploadHandler).Methods("POST")
	rtr.Handle("/_matrix/media/r0/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}", downloadHandler).Methods("GET")
	rtr.Handle("/_matrix/media/r0/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}/{filename:[a-zA-Z0-9._-]+}", downloadHandler).Methods("GET")
	rtr.Handle("/_matrix/media/r0/thumbnail/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}", thumbnailHandler).Methods("GET")
	rtr.Handle("/_matrix/media/r0/preview_url", previewUrlHandler).Methods("GET")

	// v1 endpoints (legacy)
	log.Info("Registering v1 endpoints")
	rtr.Handle("/_matrix/media/v1/upload", uploadHandler).Methods("POST")
	rtr.Handle("/_matrix/media/v1/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}", downloadHandler).Methods("GET")
	rtr.Handle("/_matrix/media/v1/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}/{filename:[a-zA-Z0-9._-]+}", downloadHandler).Methods("GET")
	rtr.Handle("/_matrix/media/v1/thumbnail/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}", thumbnailHandler).Methods("GET")
	rtr.Handle("/_matrix/media/v1/preview_url", previewUrlHandler).Methods("GET")

	// TODO: Intercept 404, 500, and 400 to respond with M_NOT_FOUND and M_UNKNOWN
	// TODO: Rate limiting (429 M_LIMIT_EXCEEDED)

	address := c.General.BindAddress + ":" + strconv.Itoa(c.General.Port)
	http.Handle("/", rtr)

	log.WithField("address", address).Info("Started up. Listening at http://" + address)
	http.ListenAndServe(address, nil)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contextLog := log.WithFields(log.Fields{
		"method":        r.Method,
		"host":          r.Host,
		"resource":      r.URL.Path,
		"contentType":   r.Header.Get("Content-Type"),
		"contentLength": r.Header.Get("Content-Length"),
		"queryString":   util.GetLogSafeQueryString(r),
		"requestId":     h.opts.reqCounter.GetNextId(),
	})
	contextLog.Info("Received request")

	// Send CORS and other basic headers
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; plugin-types application/pdf; style-src 'unsafe-inline'; object-src 'self'")
	w.Header().Set("Cache-Control", "public,max-age=86400,s-maxage=86400")
	w.Header().Set("Server", "matrix-media-repo")

	// Process response
	var res interface{} = client.AuthFailed()
	if util.IsServerOurs(r.Host, h.opts.config) {
		contextLog.Info("Server is owned by us, processing request")
		res = h.h(w, r, rcontext.RequestInfo{
			Log:     contextLog,
			Config:  h.opts.config,
			Context: r.Context(),
			Db:      h.opts.db,
		})
		if res == nil {
			res = &EmptyResponse{}
		}
	}

	b, err := json.Marshal(res)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, UnkErrJson, http.StatusInternalServerError)
		return
	}
	jsonStr := string(b)

	contextLog.Info("Replying with result: " + reflect.TypeOf(res).Elem().Name() + " " + jsonStr)

	switch result := res.(type) {
	case *client.ErrorResponse:
		w.Header().Set("Content-Type", "application/json")
		switch result.InternalCode {
		case "M_UNKNOWN_TOKEN":
			http.Error(w, jsonStr, http.StatusForbidden)
			break
		case "M_NOT_FOUND":
			http.Error(w, jsonStr, http.StatusNotFound)
			break
		case "M_MEDIA_TOO_LARGE":
			http.Error(w, jsonStr, http.StatusRequestEntityTooLarge)
			break
		case "M_BAD_REQUEST":
			http.Error(w, jsonStr, http.StatusBadRequest)
			break
		default: // M_UNKNOWN
			http.Error(w, jsonStr, http.StatusInternalServerError)
			break
		}
		break
	case *r0.DownloadMediaResponse:
		w.Header().Set("Content-Type", result.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename=\""+result.Filename+"\"")
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

func (c *requestCounter) GetNextId() string {
	strId := strconv.Itoa(c.lastId)
	c.lastId = c.lastId + 1

	return "REQ-" + strId
}
