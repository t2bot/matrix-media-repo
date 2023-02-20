package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_responses"
)

func buildPrimaryRouter() *httprouter.Router {
	router := httprouter.New()
	router.RedirectTrailingSlash = false // spec compliance
	router.RedirectFixedPath = false     // don't fix case
	router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedFn)
	router.NotFound = http.HandlerFunc(notFoundFn)
	//router.GlobalOPTIONS = http.HandlerFunc(corsFn)
	router.PanicHandler = panicFn
	return router
}

func methodNotAllowedFn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	if b, err := json.Marshal(_responses.MethodNotAllowed()); err != nil {
		panic(errors.New("error preparing MethodNotAllowed: " + err.Error()))
	} else {
		if _, err = w.Write(b); err != nil {
			panic(errors.New("error sending MethodNotAllowed: " + err.Error()))
		}
	}
}

func notFoundFn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	if b, err := json.Marshal(_responses.NotFoundError()); err != nil {
		panic(errors.New("error preparing NotFound: " + err.Error()))
	} else {
		if _, err = w.Write(b); err != nil {
			panic(errors.New("error sending NotFound: " + err.Error()))
		}
	}
}

//func corsFn(w http.ResponseWriter, r *http.Request) {
//	header := w.Header()
//	if header.Get("Access-Control-Request-Method") != "" {
//		header.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
//		header.Set("Access-Control-Allow-Origin", "Access-Control-Allow-Origin")
//	}
//
//	w.WriteHeader(http.StatusNoContent)
//}

func panicFn(w http.ResponseWriter, r *http.Request, i interface{}) {
	logrus.Errorf("Panic received on %s %s: %s", r.Method, r.URL.String(), i)

	if e, ok := i.(error); ok {
		sentry.CaptureException(e)
	} else {
		sentry.CaptureMessage(fmt.Sprintf("Unknown panic received: %T %s %+v", i, i, i))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	if b, err := json.Marshal(_responses.InternalServerError("unexpected error")); err != nil {
		panic(errors.New("error preparing InternalServerError: " + err.Error()))
	} else {
		if _, err = w.Write(b); err != nil {
			panic(errors.New("error sending InternalServerError: " + err.Error()))
		}
	}
}
