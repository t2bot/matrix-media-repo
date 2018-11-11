package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/util"
)

// This is a limited in-memory homeserver used for the purposes of facilitating the tests

type user struct {
	localpart string
	domain    string
	mxid      string
}

var accessTokenMap = make(map[string]user)

func main() {
	rtr := mux.NewRouter()
	rtr.Handle("/_matrix/client/r0/register", register{}).Methods("POST")
	rtr.Handle("/_matrix/client/r0/account/whoami", whoami{}).Methods("GET")

	http.Handle("/", rtr)
	http.ListenAndServe("localhost:8001", nil)
}

type registerResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	DeviceID    string `json:"device_id"`
}
type registerRequest struct {
	Username string `json:"username"`
}

type register struct{}

func (c register) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var t registerRequest
	err := decoder.Decode(&t)
	if err != nil {
		panic(err)
	}

	token, err := util.GenerateRandomString(32)
	if err != nil {
		panic(err)
	}

	accessTokenMap[token] = user{
		localpart: t.Username,
		domain:    "example.org",
		mxid:      fmt.Sprintf("@%s:example.org", t.Username),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.Encode(registerResponse{
		UserID:      accessTokenMap[token].mxid,
		AccessToken: token,
		DeviceID:    "example",
	})
}

type whoamiResponse struct {
	UserID string `json:"user_id"`
}

type whoami struct{}

func (c whoami) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	token = token[len("Bearer "):]

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.Encode(whoamiResponse{
		UserID: accessTokenMap[token].mxid,
	})
}
