package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/didip/tollbooth"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/client/r0"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/logging"
	"github.com/turt2live/matrix-media-repo/util"
)

const UnkErrJson = `{"code":"M_UNKNOWN","message":"Unexpected error processing response"}`

type requestCounter struct {
	lastId int
}

type Handler struct {
	h    func(http.ResponseWriter, *http.Request, *log.Entry) interface{}
	opts HandlerOpts
}

type HandlerOpts struct {
	reqCounter *requestCounter
}

type ApiRoute struct {
	Method  string
	Handler Handler
}

type EmptyResponse struct{}

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	flag.Parse()

	config.Path = *configPath

	rtr := mux.NewRouter()

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	log.Info("Starting media repository...")

	counter := requestCounter{}
	hOpts := HandlerOpts{&counter}

	optionsHandler := Handler{optionsRequest, hOpts}
	uploadHandler := Handler{r0.UploadMedia, hOpts}
	downloadHandler := Handler{r0.DownloadMedia, hOpts}
	thumbnailHandler := Handler{r0.ThumbnailMedia, hOpts}
	previewUrlHandler := Handler{r0.PreviewUrl, hOpts}
	identiconHandler := Handler{r0.Identicon, hOpts}

	routes := make(map[string]*ApiRoute)
	versions := []string{"r0", "v1"} // r0 is typically clients and v1 is typically servers

	for i := 0; i < len(versions); i++ {
		version := versions[i]
		routes["/_matrix/media/"+version+"/upload"] = &ApiRoute{"POST", uploadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}"] = &ApiRoute{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}/{filename:[a-zA-Z0-9._-]+}"] = &ApiRoute{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/thumbnail/{server:[a-zA-Z0-9.:-_]+}/{mediaId:[a-zA-Z0-9]+}"] = &ApiRoute{"GET", thumbnailHandler}
		routes["/_matrix/media/"+version+"/preview_url"] = &ApiRoute{"GET", previewUrlHandler}
		routes["/_matrix/media/"+version+"/identicon/{seed:.*}"] = &ApiRoute{"GET", identiconHandler}
	}

	for routePath, opts := range routes {
		log.Info("Registering route: " + opts.Method + " " + routePath)
		rtr.Handle(routePath, opts.Handler).Methods(opts.Method)
		rtr.Handle(routePath, optionsHandler).Methods("OPTIONS")
	}

	rtr.NotFoundHandler = Handler{client.NotFoundHandler, hOpts}
	rtr.MethodNotAllowedHandler = Handler{client.MethodNotAllowedHandler, hOpts}

	var handler http.Handler
	handler = rtr
	if config.Get().RateLimit.Enabled {
		log.Info("Enabling rate limit")
		limiter := tollbooth.NewLimiter(0, nil)
		limiter.SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"})
		limiter.SetTokenBucketExpirationTTL(time.Hour)
		limiter.SetBurst(config.Get().RateLimit.BurstCount)
		limiter.SetMax(config.Get().RateLimit.RequestsPerSecond)

		b, _ := json.Marshal(client.RateLimitReached())
		limiter.SetMessage(string(b))
		limiter.SetMessageContentType("application/json")

		handler = tollbooth.LimitHandler(limiter, rtr)
	}

	address := config.Get().General.BindAddress + ":" + strconv.Itoa(config.Get().General.Port)
	http.Handle("/", handler)

	log.WithField("address", address).Info("Started up. Listening at http://" + address)
	http.ListenAndServe(address, nil)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Host = strings.Split(r.Host, ":")[0]
	if r.Header.Get("X-Forwarded-For") != "" {
		r.RemoteAddr = r.Header.Get("X-Forwarded-For")
	}
	contextLog := log.WithFields(log.Fields{
		"method":        r.Method,
		"host":          r.Host,
		"resource":      r.URL.Path,
		"contentType":   r.Header.Get("Content-Type"),
		"contentLength": r.ContentLength,
		"queryString":   util.GetLogSafeQueryString(r),
		"requestId":     h.opts.reqCounter.GetNextId(),
		"remoteAddr":    r.RemoteAddr,
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
	if util.IsServerOurs(r.Host) {
		contextLog.Info("Server is owned by us, processing request")
		res = h.h(w, r, contextLog)
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
		case "M_METHOD_NOT_ALLOWED":
			http.Error(w, jsonStr, http.StatusMethodNotAllowed)
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
		defer result.Data.Close()
		io.Copy(w, result.Data)
		break
	case *r0.IdenticonResponse:
		w.Header().Set("Content-Type", "image/png")
		io.Copy(w, result.Avatar)
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

func optionsRequest(w http.ResponseWriter, r *http.Request, log *log.Entry) interface{} {
	return &EmptyResponse{}
}
