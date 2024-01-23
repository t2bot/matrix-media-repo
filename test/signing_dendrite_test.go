package test

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/dendrite"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
	"github.com/t2bot/matrix-media-repo/util"
)

func TestDendriteSigningKeyRoundTrip(t *testing.T) {
	raw := `-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:1Pu3u3

1Pu3u3solToI2pTdsHA4wj05bANnzPwJoxPepw2he2s=
-----END MATRIX PRIVATE KEY-----
`
	original := bytes.NewBufferString(raw)
	keyVersion := "1Pu3u3"
	canonical, err := util.EncodeCanonicalJson(database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "localhost",
		"valid_until_ts":  1701584534175,
		"verify_keys": database.AnonymousJson{
			"ed25519:1Pu3u3": database.AnonymousJson{
				"key": "uBatSEbBO6u7SNwvNwVFCD4XGaS0ikPI1PMgP5nlh3c",
			},
		},
	})
	sigB64 := "ya8NhdqVGZp8vhEgmtfIdm7gIEiLpcbp/0H2m+36nto/mXLDaGulkaQB/p7iftksiboTg/yK4BAzjWO0zFz7DQ"

	if err != nil {
		t.Fatal(err)
	}

	key, err := dendrite.DecodeSigningKey(original)
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, key.KeyVersion)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(key.PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode and decode the key as MMR format and re-test signatures
	mmrBytes, err := mmr.EncodeSigningKey(key)
	assert.NoError(t, err)
	key, err = mmr.DecodeSigningKey(bytes.NewReader(mmrBytes))
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, key.KeyVersion)

	parsedSigB64 = util.EncodeUnpaddedBase64ToString(ed25519.Sign(key.PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as Dendrite and compare to test value
	enc, err := dendrite.EncodeSigningKey(key)
	assert.NoError(t, err)
	assert.Equal(t, raw, string(enc))
}

func TestMultipleDecodeOneDendriteSigningKeyRoundTrip(t *testing.T) {
	raw := `-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:1Pu3u3

1Pu3u3solToI2pTdsHA4wj05bANnzPwJoxPepw2he2s=
-----END MATRIX PRIVATE KEY-----
-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:wBZqM2

YOA4wuCQ+Wh7Vr+zmYSZpScnrgsrSAVtP89hn0zPO+s=
-----END MATRIX PRIVATE KEY-----
`
	single := `-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:1Pu3u3

1Pu3u3solToI2pTdsHA4wj05bANnzPwJoxPepw2he2s=
-----END MATRIX PRIVATE KEY-----
`
	original := bytes.NewBufferString(raw)
	keyVersion := "1Pu3u3"
	canonical, err := util.EncodeCanonicalJson(database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "localhost",
		"valid_until_ts":  1701584534175,
		"verify_keys": database.AnonymousJson{
			"ed25519:1Pu3u3": database.AnonymousJson{
				"key": "uBatSEbBO6u7SNwvNwVFCD4XGaS0ikPI1PMgP5nlh3c",
			},
		},
	})
	sigB64 := "ya8NhdqVGZp8vhEgmtfIdm7gIEiLpcbp/0H2m+36nto/mXLDaGulkaQB/p7iftksiboTg/yK4BAzjWO0zFz7DQ"

	if err != nil {
		t.Fatal(err)
	}

	key, err := dendrite.DecodeSigningKey(original)
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, key.KeyVersion)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(key.PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode and decode the key as MMR format and re-test signatures
	mmrBytes, err := mmr.EncodeSigningKey(key)
	assert.NoError(t, err)
	key, err = mmr.DecodeSigningKey(bytes.NewReader(mmrBytes))
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, key.KeyVersion)

	parsedSigB64 = util.EncodeUnpaddedBase64ToString(ed25519.Sign(key.PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as Dendrite and compare to test value
	enc, err := dendrite.EncodeSigningKey(key)
	assert.NoError(t, err)
	assert.Equal(t, single, string(enc))
}

func TestMultipleDecodeDendriteSigningKeyRoundTrip(t *testing.T) {
	raw := `-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:1Pu3u3

1Pu3u3solToI2pTdsHA4wj05bANnzPwJoxPepw2he2s=
-----END MATRIX PRIVATE KEY-----
-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:wBZqM2

YOA4wuCQ+Wh7Vr+zmYSZpScnrgsrSAVtP89hn0zPO+s=
-----END MATRIX PRIVATE KEY-----
`
	original := bytes.NewBufferString(raw)
	keyVersion1 := "1Pu3u3"
	keyVersion2 := "wBZqM2"
	canonical, err := util.EncodeCanonicalJson(database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "localhost",
		"valid_until_ts":  1701584534175,
		"verify_keys": database.AnonymousJson{
			"ed25519:1Pu3u3": database.AnonymousJson{
				"key": "uBatSEbBO6u7SNwvNwVFCD4XGaS0ikPI1PMgP5nlh3c",
			},
		},
	})
	sigB64 := "ya8NhdqVGZp8vhEgmtfIdm7gIEiLpcbp/0H2m+36nto/mXLDaGulkaQB/p7iftksiboTg/yK4BAzjWO0zFz7DQ"

	if err != nil {
		t.Fatal(err)
	}

	keys, err := dendrite.DecodeAllSigningKeys(original)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(keys))
	assert.Equal(t, keyVersion1, keys[0].KeyVersion)
	assert.Equal(t, keyVersion2, keys[1].KeyVersion)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(keys[0].PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode and decode the key as MMR format and re-test signatures
	mmrBytes, err := mmr.EncodeAllSigningKeys(keys)
	assert.NoError(t, err)
	keys, err = mmr.DecodeAllSigningKeys(bytes.NewReader(mmrBytes))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(keys))
	assert.Equal(t, keyVersion1, keys[0].KeyVersion)
	assert.Equal(t, keyVersion2, keys[1].KeyVersion)

	parsedSigB64 = util.EncodeUnpaddedBase64ToString(ed25519.Sign(keys[0].PrivateKey, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as Dendrite and compare to test value
	enc, err := dendrite.EncodeAllSigningKeys(keys)
	assert.NoError(t, err)
	assert.Equal(t, raw, string(enc))
}
