package common

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/dendrite"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/synapse"
)

func EncodeSigningKeys(keys []*homeserver_interop.SigningKey, format string, file string) {
	var err error
	var b []byte
	switch format {
	case "synapse":
		b, err = synapse.EncodeAllSigningKeys(keys)
	case "dendrite":
		b, err = dendrite.EncodeAllSigningKeys(keys)
	case "mmr":
		b, err = mmr.EncodeAllSigningKeys(keys)
	default:
		logrus.Fatalf("Unknown output format '%s'. Try '%s -help' for information.", format, os.Args[0])
	}
	if err != nil {
		logrus.Fatal(err)
	}

	f, err := os.Create(file)
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

	logrus.Infof("Done! Signing key written to '%s' in %s format", f.Name(), format)
}
