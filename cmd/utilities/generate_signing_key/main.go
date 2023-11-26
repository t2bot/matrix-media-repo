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
	"github.com/turt2live/matrix-media-repo/homeserver_interop/any_server"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/dendrite"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/mmr"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
)

func main() {
	inputFile := flag.String("input", "", "When set to a file path, the signing key to convert to the output format. The key must have been generated in a format supported by -format.")
	outputFormat := flag.String("format", "mmr", "The output format for the key. May be 'mmr', 'synapse', or 'dendrite'.")
	outputFile := flag.String("output", "./signing.key", "The output file for the key.")
	flag.Parse()

	var keyVersion string
	var priv ed25519.PrivateKey
	var err error

	if *inputFile != "" {
		priv, keyVersion, err = decodeKey(*inputFile)
	} else {
		keyVersion = makeKeyVersion()
		_, priv, err = ed25519.GenerateKey(nil)
		priv = priv[len(priv)-32:]
	}
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Key ID will be 'ed25519:%s'", keyVersion)

	var b []byte
	switch *outputFormat {
	case "synapse":
		b, err = synapse.EncodeSigningKey(keyVersion, priv)
		break
	case "dendrite":
		b, err = dendrite.EncodeSigningKey(keyVersion, priv)
		break
	case "mmr":
		b, err = mmr.EncodeSigningKey(keyVersion, priv)
		break
	default:
		logrus.Fatalf("Unknown output format '%s'. Try '%s -help' for information.", *outputFormat, flag.Arg(0))
	}

	f, err := os.Create(*outputFile)
	if err != nil {
		logrus.Fatal(err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err = f.Write(b)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Done! Signing key written to '%s' in %s format", f.Name(), *outputFormat)
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

func decodeKey(fileName string) (ed25519.PrivateKey, string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	return any_server.DecodeSigningKey(f)
}
