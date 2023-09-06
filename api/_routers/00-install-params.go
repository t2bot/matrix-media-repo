package _routers

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/julienschmidt/httprouter"
)

func localCompile(expr string) *regexp.Regexp {
	r, err := regexp.Compile(expr)
	if err != nil {
		panic(errors.New("error compiling expression: " + expr + " | " + err.Error()))
	}
	return r
}

var ServerNameRegex = localCompile("[a-zA-Z0-9.:\\-_]+")

//var NumericIdRegex = localCompile("[0-9]+")

func GetParam(name string, r *http.Request) string {
	p := httprouter.ParamsFromContext(r.Context())
	if p == nil {
		return ""
	}
	return p.ByName(name)
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
