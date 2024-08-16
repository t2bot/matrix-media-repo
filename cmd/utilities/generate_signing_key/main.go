package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/cmd/utilities/_common"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/any_server"
)

func main() {
	inputFile := flag.String("input", "", "When set to a file path, the signing key to convert to the output format. The key must have been generated in a format supported by -format. If the format supports multiple keys, only the first will be converted.")
	outputFormat := flag.String("format", "mmr", "The output format for the key. May be 'mmr', 'synapse', or 'dendrite'.")
	outputFile := flag.String("output", "./signing.key", "The output file for the key.")
	flag.Parse()

	var key *homeserver_interop.SigningKey
	var err error

	if *inputFile != "" {
		key, err = decodeKey(*inputFile)
	} else {
		key, err = homeserver_interop.GenerateSigningKey()
	}
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Key ID will be 'ed25519:%s'", key.KeyVersion)

	_common.EncodeSigningKeys([]*homeserver_interop.SigningKey{key}, *outputFormat, *outputFile)
}

func decodeKey(fileName string) (*homeserver_interop.SigningKey, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return any_server.DecodeSigningKey(f)
}
