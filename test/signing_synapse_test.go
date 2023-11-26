package test

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/mmr"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
	"github.com/turt2live/matrix-media-repo/util"
)

func TestSynapseSigningKeyRoundTrip(t *testing.T) {
	raw := "ed25519 a_RVfN wdSWsTNSOmMuNA1Ej6JUyeNbiBEt5jexHmVs7mHKZVc"
	original := bytes.NewBufferString(raw)
	keyVersion := "a_RVfN"
	canonical, err := util.EncodeCanonicalJson(database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "localhost",
		"valid_until_ts":  1701065483311,
		"verify_keys": database.AnonymousJson{
			"ed25519:a_RVfN": database.AnonymousJson{
				"key": "N1wtx39rXnuCsbjt3Yhrb0jkPI/zI5kphIV9ai1enW0",
			},
		},
	})
	sigB64 := "hCcSfyiyMPZU93ysk+r62aC0nkbUKRgzwzRpPO85VUshILT64fg5mPykMUb/XU0G3Tr7/Qn8uTpdPkoZ3B+QDw"

	if err != nil {
		t.Fatal(err)
	}

	parsedPriv, parsedKeyVer, err := synapse.DecodeSigningKey(original)
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, parsedKeyVer)

	parsedSigB64 := util.EncodeUnpaddedBase64ToString(ed25519.Sign(parsedPriv, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode and decode the key as MMR format and re-test signatures
	mmrBytes, err := mmr.EncodeSigningKey(parsedKeyVer, parsedPriv)
	assert.NoError(t, err)
	parsedPriv, parsedKeyVer, err = mmr.DecodeSigningKey(bytes.NewReader(mmrBytes))
	assert.NoError(t, err)
	assert.Equal(t, keyVersion, parsedKeyVer)

	parsedSigB64 = util.EncodeUnpaddedBase64ToString(ed25519.Sign(parsedPriv, canonical))
	assert.Equal(t, sigB64, parsedSigB64)

	// Encode as Synapse and compare to test value
	enc, err := synapse.EncodeSigningKey(parsedKeyVer, parsedPriv)
	assert.NoError(t, err)
	assert.Equal(t, raw, string(enc))
}
