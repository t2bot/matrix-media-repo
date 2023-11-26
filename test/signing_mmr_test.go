package test

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/mmr"
	"github.com/turt2live/matrix-media-repo/util"
)

func TestMMRSigningKeyRoundTrip(t *testing.T) {
	raw := `-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:e5d0oC
Version: 1

PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgc=
-----END MMR PRIVATE KEY-----
`
	original := bytes.NewBufferString(raw)
	keyVersion := "e5d0oC"
	canonical, err := util.EncodeCanonicalJson(database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "localhost",
		"valid_until_ts":  1700979986627,
		"verify_keys": database.AnonymousJson{
			"ed25519:e5d0oC": database.AnonymousJson{
				"key": "TohekYXzLx7VzV8FtLQlI3XsSdPv1CjhVYY5rZmFCvU",
			},
		},
	})
	sigB64 := "FRIe4KJ5kdnBXJgQCgC057YcHafHZmidNqYtSSWLU7QMgDu8uMHWcuPPack8zys1GeLdgS9d5YolmyOVQT9WDA"

	if err != nil {
		t.Fatal(err)
	}

	parsedPriv, parsedKeyVer, err := mmr.DecodeSigningKey(original)
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, parsedKeyVer)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(parsedPriv, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as MMR again and compare to test value
	enc, err := mmr.EncodeSigningKey(parsedKeyVer, parsedPriv)
	assert.NoError(t, err)
	assert.Equal(t, raw, string(enc))
}
