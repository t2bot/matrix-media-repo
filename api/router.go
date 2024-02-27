package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
)

func buildPrimaryRouter() *httprouter.Router {
	router := httprouter.New()
	router.RedirectTrailingSlash = false // spec compliance
	router.RedirectFixedPath = false     // don't fix case
	router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedFn)
	router.NotFound = http.HandlerFunc(notFoundFn)
	router.HandleOPTIONS = true
	router.GlobalOPTIONS = _routers.NewInstallHeadersRouter(http.HandlerFunc(finishCorsFn))
	router.PanicHandler = panicFn
	return router
}

func methodNotAllowedFn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	b, err := json.Marshal(_responses.MethodNotAllowed())
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error preparing MethodNotAllowed: %v", err))
		logrus.Errorf("error preparing MethodNotAllowed: %v", err)
		return
	}
	w.Write(b)
}

func notFoundFn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	b, err := json.Marshal(_responses.NotFoundError())
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error preparing NotFound: %v", err))
		logrus.Errorf("error preparing NotFound: %v", err)
		return
	}
	w.Write(b)
}

func finishCorsFn(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func panicFn(w http.ResponseWriter, r *http.Request, i interface{}) {
	logrus.Errorf("Panic received on %s %s: %s", r.Method, util.GetLogSafeUrl(r), i)

	//goland:noinspection GoTypeAssertionOnErrors
	if e, ok := i.(error); ok {
		sentry.CaptureException(e)
	} else {
		sentry.CaptureMessage(fmt.Sprintf("Unknown panic received: %T %s %+v", i, i, i))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	b, err := json.Marshal(_responses.InternalServerError(errors.New("unexpected error")))
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error preparing InternalServerError: %v", err))
		logrus.Errorf("error preparing InternalServerError: %v", err)
		return
	}
	w.Write(b)
}
