package main

import (
	"flag"
	"fmt"

	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/synapse"
)

func main() {
	c, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}

	db, err := storage.OpenDatabase(c.Database.Postgres)
	if err != nil {
		panic(err)
	}

	homeserverYamlPath := flag.String("homeserver", "homeserver.yaml", "Path to your homeserver.yaml")
	moveFiles := flag.Bool("move", false, "If set, files will be moved instead of copied")
	importRemote := flag.Bool("remote", false, "If set, remote media will also be imported")
	flag.Parse()

	hsConfig, err := synapse.ReadConfig(*homeserverYamlPath)
	if err != nil {
		panic(err)
	}

	fmt.Println(*moveFiles)
	fmt.Println(*importRemote)
	fmt.Println(*db)
	fmt.Println(hsConfig.GetConnectionString())
}