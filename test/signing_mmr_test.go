package test

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
	"github.com/t2bot/matrix-media-repo/util"
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

	key, err := mmr.DecodeSigningKey(original)
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, key.KeyVersion)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(key.PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as MMR again and compare to test value
	enc, err := mmr.EncodeSigningKey(key)
	assert.NoError(t, err)
	assert.Equal(t, raw, string(enc))
}

func TestMultipleDecodeOneMMRSigningKeyRoundTrip(t *testing.T) {
	raw := `-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:e5d0oC
Version: 1

PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgc=
-----END MMR PRIVATE KEY-----
-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:CjtBYA
Version: 1

Xj/gSXAtSOTPG54sAWjm9BMVNX/Pa8ujIzwYbvLk7mM=
-----END MMR PRIVATE KEY-----
`
	single := `-----BEGIN MMR PRIVATE KEY-----
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

	key, err := mmr.DecodeSigningKey(original)
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, key.KeyVersion)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(key.PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as MMR again and compare to test value
	enc, err := mmr.EncodeSigningKey(key)
	assert.NoError(t, err)
	assert.Equal(t, single, string(enc))
}

func TestMultipleDecodeMMRSigningKeyRoundTrip(t *testing.T) {
	raw := `-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:e5d0oC
Version: 1

PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgc=
-----END MMR PRIVATE KEY-----
-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:CjtBYA
Version: 1

Xj/gSXAtSOTPG54sAWjm9BMVNX/Pa8ujIzwYbvLk7mM=
-----END MMR PRIVATE KEY-----
`
	original := bytes.NewBufferString(raw)
	keyVersion1 := "e5d0oC"
	keyVersion2 := "CjtBYA"
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

	keys, err := mmr.DecodeAllSigningKeys(original)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(keys))
	assert.Equal(t, keyVersion1, keys[0].KeyVersion)
	assert.Equal(t, keyVersion2, keys[1].KeyVersion)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(keys[0].PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as MMR again and compare to test value
	enc, err := mmr.EncodeAllSigningKeys(keys)
	assert.NoError(t, err)
	assert.Equal(t, raw, string(enc))
}
