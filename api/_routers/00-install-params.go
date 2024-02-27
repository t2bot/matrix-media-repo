package _routers

import (
	"context"
	"net/http"
	"regexp"

	"github.com/julienschmidt/httprouter"
)

var ServerNameRegex = regexp.MustCompile("[a-zA-Z0-9.:\\-_]+")

// var NumericIdRegex = regexp.MustCompile("[0-9]+")
func GetParam(name string, r *http.Request) string {
	parameter := httprouter.ParamsFromContext(r.Context())
	if parameter == nil {
		return ""
	}
	return parameter.ByName(name)
}

func ForceSetParam(name string, val string, r *http.Request) *http.Request {
	params := httprouter.ParamsFromContext(r.Context())
	wasSet := false
	for _, p := range params {
		if p.Key == name {
			p.Value = val
			wasSet = true
			break
		}
	}
	if !wasSet {
		params = append(params, httprouter.Param{
			Key:   name,
			Value: val,
		})
	}
	return r.WithContext(context.WithValue(r.Context(), httprouter.ParamsKey, params))
}
