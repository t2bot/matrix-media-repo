package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/cmd/utilities/common"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/any_server"
	"github.com/t2bot/matrix-media-repo/util"
)

func main() {
	outputFormat := flag.String("format", "mmr", "The output format for the key. May be 'mmr', 'synapse', or 'dendrite'.")
	outputFile := flag.String("output", "./signing.key", "The output file for the key. Note that not all software will use multiple keys.")
	flag.Parse()

	keys := make(map[string]*homeserver_interop.SigningKey)
	keysArray := make([]*homeserver_interop.SigningKey, 0)
	for _, file := range flag.Args() {
		logrus.Infof("Reading %s", file)

		localKeys, err := decodeKeys(file)
		if err != nil {
			logrus.Fatal(err)
		}

		for _, key := range localKeys {
			if val, ok := keys[key.KeyVersion]; ok {
				logrus.Fatalf("Duplicate key version '%s' detected. Known='%s', duplicate='%s'", key.KeyVersion, util.EncodeUnpaddedBase64ToString(val.PrivateKey), util.EncodeUnpaddedBase64ToString(key.PrivateKey))
			}

			keys[key.KeyVersion] = key
			keysArray = append(keysArray, key)
		}
	}

	common.EncodeSigningKeys(keysArray, *outputFormat, *outputFile)
}

func decodeKeys(fileName string) ([]*homeserver_interop.SigningKey, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return any_server.DecodeAllSigningKeys(f)
}
