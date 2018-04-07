package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/didip/tollbooth"
	"github.com/gorilla/mux"
	"github.com/sebest/xff"
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
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the migrations folder")
	flag.Parse()

	config.Path = *configPath
	config.Runtime.MigrationsPath = *migrationsPath

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
	purgeHandler := Handler{r0.PurgeRemoteMedia, hOpts}
	quarantineHandler := Handler{r0.QuarantineMedia, hOpts}
	quarantineRoomHandler := Handler{r0.QuarantineRoomMedia, hOpts}
	localCopyHandler := Handler{r0.LocalCopy, hOpts}
	infoHandler := Handler{r0.MediaInfo, hOpts}

	routes := make(map[string]*ApiRoute)
	versions := []string{"r0", "v1"} // r0 is typically clients and v1 is typically servers

	for _, version := range versions {
		// Standard routes for the media repo
		routes["/_matrix/media/"+version+"/upload"] = &ApiRoute{"POST", uploadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = &ApiRoute{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}/{filename:[a-zA-Z0-9._\\-]+}"] = &ApiRoute{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/thumbnail/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = &ApiRoute{"GET", thumbnailHandler}
		routes["/_matrix/media/"+version+"/preview_url"] = &ApiRoute{"GET", previewUrlHandler}
		routes["/_matrix/media/"+version+"/identicon/{seed:.*}"] = &ApiRoute{"GET", identiconHandler}

		// Custom routes for the media repo
		routes["/_matrix/media/"+version+"/admin/purge_remote"] = &ApiRoute{"POST", purgeHandler}
		routes["/_matrix/media/"+version+"/admin/quarantine/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = &ApiRoute{"POST", quarantineHandler}
		routes["/_matrix/media/"+version+"/admin/room/{roomId:[^/]+}/quarantine"] = &ApiRoute{"POST", quarantineRoomHandler}
		routes["/_matrix/media/"+version+"/local_copy/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = &ApiRoute{"GET", localCopyHandler}
		routes["/_matrix/media/"+version+"/info/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = &ApiRoute{"GET", infoHandler}

		// Routes that don't fit the normal media spec
		routes["/_matrix/client/"+version+"/admin/purge_media_cache"] = &ApiRoute{"POST", purgeHandler}
		routes["/_matrix/client/"+version+"/admin/quarantine_media/{roomId:[^/]+}"] = &ApiRoute{"POST", quarantineRoomHandler}
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
	isUsingForwardedHost := false
	if r.Header.Get("X-Forwarded-Host") != "" {
		r.Host = r.Header.Get("X-Forwarded-Host")
		isUsingForwardedHost = true
	}
	r.Host = strings.Split(r.Host, ":")[0]

	raddr := xff.GetRemoteAddr(r)
	host, _, err := net.SplitHostPort(raddr)
	if err != nil {
		log.Error(err)
		host = raddr
	}
	r.RemoteAddr = host

	contextLog := log.WithFields(log.Fields{
		"method":             r.Method,
		"host":               r.Host,
		"usingForwardedHost": isUsingForwardedHost,
		"resource":           r.URL.Path,
		"contentType":        r.Header.Get("Content-Type"),
		"contentLength":      r.ContentLength,
		"queryString":        util.GetLogSafeQueryString(r),
		"requestId":          h.opts.reqCounter.GetNextId(),
		"remoteAddr":         r.RemoteAddr,
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
	jsonStr := UnkErrJson
	if err == nil {
		jsonStr = string(b)
	}

	contextLog.Info("Replying with result: " + reflect.TypeOf(res).Elem().Name() + " " + jsonStr)

	statusCode := http.StatusOK
	switch result := res.(type) {
	case *client.ErrorResponse:
		switch result.InternalCode {
		case "M_UNKNOWN_TOKEN":
			statusCode = http.StatusForbidden
			break
		case "M_NOT_FOUND":
			statusCode = http.StatusNotFound
			break
		case "M_MEDIA_TOO_LARGE":
			statusCode = http.StatusRequestEntityTooLarge
			break
		case "M_BAD_REQUEST":
			statusCode = http.StatusBadRequest
			break
		case "M_METHOD_NOT_ALLOWED":
			statusCode = http.StatusMethodNotAllowed
			break
		default: // M_UNKNOWN
			statusCode = http.StatusInternalServerError
			break
		}
		break
	case *r0.DownloadMediaResponse:
		w.Header().Set("Content-Type", result.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename=\""+result.Filename+"\"")
		w.Header().Set("Content-Length", fmt.Sprint(result.SizeBytes))
		defer result.Data.Close()
		io.Copy(w, result.Data)
		return // Prevent sending conflicting responses
	case *r0.IdenticonResponse:
		w.Header().Set("Content-Type", "image/png")
		io.Copy(w, result.Avatar)
		return // Prevent sending conflicting responses
	default:
		break
	}

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, jsonStr)
}

func (c *requestCounter) GetNextId() string {
	strId := strconv.Itoa(c.lastId)
	c.lastId = c.lastId + 1

	return "REQ-" + strId
}

func optionsRequest(w http.ResponseWriter, r *http.Request, log *log.Entry) interface{} {
	return &EmptyResponse{}
}
