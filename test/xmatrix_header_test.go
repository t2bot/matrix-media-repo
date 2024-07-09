package test

import (
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/util"
)

func TestXMatrixAuthHeader(t *testing.T) {
	body := []byte(nil)

	config.AddDomainForTesting("localhost", nil)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	header, err := matrix.CreateXMatrixHeader("localhost:8008", "localhost", "GET", "/_matrix/media/v3/download/example.org/abc", body, priv, "0")
	if err != nil {
		t.Fatal(err)
	}

	auths, err := util.GetXMatrixAuth([]string{header})
	if err != nil {
		t.Fatal(err)
	}

	keys := make(matrix.ServerSigningKeys)
	keys["ed25519:0"] = pub
	err = matrix.ValidateXMatrixAuthHeader("GET", "/_matrix/media/v3/download/example.org/abc", body, auths, keys, "localhost")
	assert.NoError(t, err)
}

func TestXMatrixAuthDestinationMismatch(t *testing.T) {
	body := []byte(nil)

	config.AddDomainForTesting("localhost", nil)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	header, err := matrix.CreateXMatrixHeader("localhost:8008", "localhost:1234", "GET", "/_matrix/media/v3/download/example.org/abc", body, priv, "0")
	if err != nil {
		t.Fatal(err)
	}

	auths, err := util.GetXMatrixAuth([]string{header})
	if err != nil {
		t.Fatal(err)
	}

	keys := make(matrix.ServerSigningKeys)
	keys["ed25519:0"] = pub
	err = matrix.ValidateXMatrixAuthHeader("GET", "/_matrix/media/v3/download/example.org/abc", body, auths, keys, "localhost:1234")
	assert.ErrorIs(t, err, matrix.ErrWrongDestination)
}
