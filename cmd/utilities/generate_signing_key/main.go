package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/cmd/utilities/_common"
	"github.com/turt2live/matrix-media-repo/homeserver_interop"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/any_server"
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
		keyVersion := makeKeyVersion()

		var priv ed25519.PrivateKey
		_, priv, err = ed25519.GenerateKey(nil)
		priv = priv[len(priv)-32:]

		key = &homeserver_interop.SigningKey{
			PrivateKey: priv,
			KeyVersion: keyVersion,
		}
	}
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Key ID will be 'ed25519:%s'", key.KeyVersion)

	_common.EncodeSigningKeys([]*homeserver_interop.SigningKey{key}, *outputFormat, *outputFile)
}

func makeKeyVersion() string {
	buf := make([]byte, 2)
	chars := strings.Split("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "")
	for i := 0; i < len(chars); i++ {
		sort.Slice(chars, func(i int, j int) bool {
			c, err := rand.Read(buf)

			// "should never happen" clauses
			if err != nil {
				panic(err)
			}
			if c != len(buf) || c != 2 {
				panic(fmt.Sprintf("crypto rand read %d bytes, expected %d", c, len(buf)))
			}

			return buf[0] < buf[1]
		})
	}

	return strings.Join(chars[:6], "")
}

func decodeKey(fileName string) (*homeserver_interop.SigningKey, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return any_server.DecodeSigningKey(f)
}
