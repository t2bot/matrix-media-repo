package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	http.HandleFunc("/_matrix/client/r0/account/whoami", whoami)
	http.HandleFunc("/_matrix/media/r0/download/example.org/file1", downloadFile1)
	http.HandleFunc("/_matrix/media/r0/download/example.org/file2", downloadFile2)
	http.HandleFunc("/_matrix/media/r0/download/example.org/large", downloadLargeFile)
	http.HandleFunc("/_matrix/media/r0/download/example.org/notfound", notFound)
	if err := http.ListenAndServe(":9000", nil); err != nil {
		panic(err)
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"Not found"}`))
}

func whoami(w http.ResponseWriter, r *http.Request) {
	accessToken := r.URL.Query().Get("access_token")
	userId := r.URL.Query().Get("user_id")

	resp := "{}"
	code := http.StatusOK
	if accessToken == "test-appservice" {
		if userId == "@appservice:localhost" {
			resp = fmt.Sprintf(`{"user_id":"%s"}`, userId)
		} else {
			resp = `{"errcode":"M_UNKNOWN_TOKEN","error":"Unrecognised access token"}`
			code = http.StatusUnauthorized
		}
	} else if accessToken == "test-1" {
		resp = fmt.Sprintf(`{"user_id":"%s"}`, "@test1:localhost")
	} else if accessToken == "test-2" {
		resp = fmt.Sprintf(`{"user_id":"%s"}`, "@test2:localhost")
	} else {
		resp = `{"errcode":"M_UNKNOWN_TOKEN","error":"Unrecognised access token"}`
		code = http.StatusUnauthorized
	}

	w.WriteHeader(code)
	w.Write([]byte(resp))
}

func downloadFile1(w http.ResponseWriter, r *http.Request) {
	downloadFile(w, r, "file1.txt", "text/plain")
}

func downloadFile2(w http.ResponseWriter, r *http.Request) {
	downloadFile(w, r, "file2.png", "image/png")
}

func downloadLargeFile(w http.ResponseWriter, r *http.Request) {
	downloadFile(w, r, "large.txt", "text/plain")
}

func downloadFile(w http.ResponseWriter, r *http.Request, p string, ctype string) {
	data, err := ioutil.ReadFile("tests/res/" + p)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}