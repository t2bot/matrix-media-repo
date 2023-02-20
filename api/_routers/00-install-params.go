package _routers

import (
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
