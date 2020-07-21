package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/gorilla/mux"
)

func main() {
	// Prepare local server
	log.Println("Preparing local server...")
	rtr := mux.NewRouter()
	rtr.HandleFunc("/_matrix/client/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, err := w.Write([]byte("{\"versions\":[\"r0.6.0\"]}"))
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
