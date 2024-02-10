package test

import (
	"crypto/ed25519"
	"testing"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/util"
)

func TestXMatrixAuthHeader(t *testing.T) {
	config.AddDomainForTesting("localhost", nil)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	header, err := matrix.CreateXMatrixHeader("localhost:8008", "localhost", "GET", "/_matrix/media/v3/download/example.org/abc", &database.AnonymousJson{}, &priv, "0")
	if err != nil {
		t.Fatal(err)
	}

	auths, err := util.GetXMatrixAuth([]string{header})
	if err != nil {
		t.Fatal(err)
	}

	keys := make(matrix.ServerSigningKeys)
	keys["ed25519:0"] = pub
	err = matrix.ValidateXMatrixAuthHeader("GET", "/_matrix/media/v3/download/example.org/abc", &database.AnonymousJson{}, auths, keys)
	if err != nil {
		t.Error(err)
	}
}
