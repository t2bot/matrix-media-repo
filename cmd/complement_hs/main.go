package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type VersionsResponse struct {
	CSAPIVersions []string `json:"versions,flow"`
}

type RegisterRequest struct {
	DesiredUsername string `json:"username"`
}

type RegisterResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
}

type WhoamiResponse struct {
	UserID string `json:"user_id"`
}

func requestJson(r *http.Request, i interface{}) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &i)
}

func respondJson(w http.ResponseWriter, i interface{}) error {
	resp, err := json.Marshal(i)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(resp)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err = w.Write(resp)
	return err
}

func main() {
	// Prepare local server
	log.Println("Preparing local server...")
	rtr := mux.NewRouter()
	rtr.HandleFunc("/_matrix/client/versions", func(w http.ResponseWriter, r *http.Request) {
		defer cleanup.DumpAndCloseStream(r.Body)
		err := respondJson(w, &VersionsResponse{CSAPIVersions: []string{"r0.6.0"}})
		if err != nil {
			log.Fatal(err)
		}
	})
	rtr.HandleFunc("/_matrix/client/r0/register", func(w http.ResponseWriter, r *http.Request) {
		rr := &RegisterRequest{}
		err := requestJson(r, &rr)
		if err != nil {
			log.Fatal(err)
		}
		userId := fmt.Sprintf("@%s:%s", rr.DesiredUsername, os.Getenv("SERVER_NAME"))
		err = respondJson(w, &RegisterResponse{
			AccessToken: userId,
			UserID:      userId,
		})
		if err != nil {
			log.Fatal(err)
		}
	})
	rtr.HandleFunc("/_matrix/client/r0/account/whoami", func(w http.ResponseWriter, r *http.Request) {
		defer cleanup.DumpAndCloseStream(r.Body)
		userId := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") // including space after Bearer.
		err := respondJson(w, &WhoamiResponse{UserID: userId})
		if err != nil {
			log.Fatal(err)
		}
	})
	rtr.PathPrefix("/_matrix/media/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Proxy to the media repo running within the container
		defer cleanup.DumpAndCloseStream(r.Body)
		r2, err := http.NewRequest(r.Method, "http://127.0.0.1:8228"+r.RequestURI, r.Body)
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range r.Header {
			r2.Header.Set(k, v[0])
		}
		r2.Host = os.Getenv("SERVER_NAME")
		resp, err := http.DefaultClient.Do(r2)
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range resp.Header {
			w.Header().Set(k, v[0])
		}
		defer cleanup.DumpAndCloseStream(resp.Body)
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Fatal(err)
		}
	})

	srv1 := &http.Server{Addr: "0.0.0.0:8008", Handler: rtr}
	srv2 := &http.Server{Addr: "0.0.0.0:8448", Handler: rtr}

	log.Println("Starting local server...")
	waitGroup1 := &sync.WaitGroup{}
	waitGroup2 := &sync.WaitGroup{}
	go func() {
		if err := srv1.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
		srv1 = nil
		waitGroup1.Done()
	}()
	go func() {
		if err := srv2.ListenAndServeTLS("/data/server.crt", "/data/server.key"); err != http.ErrServerClosed {
			log.Fatal(err)
		}
		srv2 = nil
		waitGroup2.Done()
	}()

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, os.Kill)
	go func() {
		defer close(stop)
		<-stop
		log.Println("Stopping local server...")
		_ = srv1.Close()
		_ = srv2.Close()
	}()

	waitGroup1.Add(1)
	waitGroup2.Add(1)
	waitGroup1.Wait()
	waitGroup2.Wait()

	log.Println("Goodbye!")
}
