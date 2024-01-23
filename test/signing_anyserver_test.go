package test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/any_server"
	"github.com/t2bot/matrix-media-repo/util"
)

func TestAnyServerDecodeDendrite(t *testing.T) {
	raw := `-----BEGIN MATRIX PRIVATE KEY-----
Key-ID: ed25519:1Pu3u3

1Pu3u3solToI2pTdsHA4wj05bANnzPwJoxPepw2he2s=
-----END MATRIX PRIVATE KEY-----
`

	key, err := any_server.DecodeSigningKey(bytes.NewReader([]byte(raw)))
	assert.NoError(t, err)
	assert.Equal(t, "1Pu3u3", key.KeyVersion)
	assert.Equal(t, "1Pu3u3solToI2pTdsHA4wj05bANnzPwJoxPepw2he2u4Fq1IRsE7q7tI3C83BUUIPhcZpLSKQ8jU8yA/meWHdw", util.EncodeUnpaddedBase64ToString(key.PrivateKey))
}

func TestAnyServerDecodeSynapse(t *testing.T) {
	raw := `ed25519 a_RVfN wdSWsTNSOmMuNA1Ej6JUyeNbiBEt5jexHmVs7mHKZVc`

	key, err := any_server.DecodeSigningKey(bytes.NewReader([]byte(raw)))
	assert.NoError(t, err)
	assert.Equal(t, "a_RVfN", key.KeyVersion)
	assert.Equal(t, "wdSWsTNSOmMuNA1Ej6JUyeNbiBEt5jexHmVs7mHKZVc3XC3Hf2tee4KxuO3diGtvSOQ8j/MjmSmEhX1qLV6dbQ", util.EncodeUnpaddedBase64ToString(key.PrivateKey))
}

func TestAnyServerDecodeMMR(t *testing.T) {
	raw := `-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:e5d0oC
Version: 1

PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgc=
-----END MMR PRIVATE KEY-----
`

	key, err := any_server.DecodeSigningKey(bytes.NewReader([]byte(raw)))
	assert.NoError(t, err)
	assert.Equal(t, "e5d0oC", key.KeyVersion)
	assert.Equal(t, "PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgdOiF6RhfMvHtXNXwW0tCUjdexJ0+/UKOFVhjmtmYUK9Q", util.EncodeUnpaddedBase64ToString(key.PrivateKey))
}
