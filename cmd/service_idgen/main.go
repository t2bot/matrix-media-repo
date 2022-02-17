package main

import (
	"flag"
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/util"
	"net/http"
	"os"
	"strconv"
)

func main() {
	machineId := flag.Int("machine", getIdFromEnv(), "The machine ID. 0-1023 (inclusive)")
	secret := flag.String("secret", getValFromEnv("API_SECRET", ""), "The API secret to require on requests")
	bind := flag.String("bind", getValFromEnv("API_BIND", ":8090"), "Where to bind the API to")
	flag.Parse()

	node, err := snowflake.NewNode(int64(*machineId))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Running as machine %d\n", *machineId)

	expectedSecret := fmt.Sprintf("Bearer %s", *secret)

	http.HandleFunc("/v1/id", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != expectedSecret {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Generate a random string to pad out the returned ID
		r, err := util.GenerateRandomString(32)
		if err != nil {
			logrus.Error(err)
			w.WriteHeader(500)
			return
		}
		s := r + node.Generate().String()

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(s))
		if err != nil {
			fmt.Println(err)
			return
		}
	})

	err = http.ListenAndServe(*bind, nil)
	if err != nil {
		panic(err)
	}
}

func getIdFromEnv() int {
	if val, ok := os.LookupEnv("MACHINE_ID"); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}

func getValFromEnv(key string, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}
