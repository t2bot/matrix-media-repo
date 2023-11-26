package test

import (
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

func TestFailInjectedKeys(t *testing.T) {
	raw := database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "x.resolvematrix.dev",
		"signatures": database.AnonymousJson{
			"x.resolvematrix.dev": database.AnonymousJson{
				"ed25519:injected": "FB93YAF+fOPyWcsx285Q/xFzRiG5sr7/u1iX9XWaIcOwDyDDwx7daS1eYxuM9PfosWE5vqUyTsCxmB40JTzdCw",
			},
		},
		"valid_until_ts": 1701055573679,
		"verify_keys": database.AnonymousJson{
			"ed25519:AY4k3ADlto8": database.AnonymousJson{"key": "VF7dl9W/tFWAjZSXm42Ef22k3v4WKBYLXZF9I7ErU00"},
			"ed25519:injected":    database.AnonymousJson{"key": "w48CLiV1IkWoEbqJLFmniGUYtxwT+c2zm87X8oEpRO8"},
		},
	}
	keyInfo := new(matrix.ServerKeyResult)
	err := raw.ApplyTo(keyInfo)
	if err != nil {
		t.Fatal(err)
	}

	_, err = matrix.CheckSigningKeySignatures("x.resolvematrix.dev", keyInfo, raw)
	assert.Error(t, err)
	assert.Equal(t, "missing signature from key 'ed25519:AY4k3ADlto8'", err.Error())
}

func TestRegularKeys(t *testing.T) {
	raw := database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     "x.resolvematrix.dev",
		"signatures": database.AnonymousJson{
			"x.resolvematrix.dev": database.AnonymousJson{
				"ed25519:AY4k3ADlto8": "3WlsmHFTVjywCoDYyrtx3ies+VufTuBuw1Prlgmoqh+a4XrJT+isEwhTX+I5FBvtJTKTt6vLH3gaP7BA6712CA",
			},
		},
		"valid_until_ts": 1701057124839,
		"verify_keys": database.AnonymousJson{
			"ed25519:AY4k3ADlto8": database.AnonymousJson{"key": "VF7dl9W/tFWAjZSXm42Ef22k3v4WKBYLXZF9I7ErU00"},
		},
	}
	keyInfo := new(matrix.ServerKeyResult)
	err := raw.ApplyTo(keyInfo)
	if err != nil {
		t.Fatal(err)
	}

	keys, err := matrix.CheckSigningKeySignatures("x.resolvematrix.dev", keyInfo, raw)
	assert.NoError(t, err)
	for keyId, keyVal := range keys {
		if b64, ok := keyInfo.VerifyKeys[keyId]; !ok {
			t.Errorf("got key for '%s' but wasn't expecting it", keyId)
		} else {
			keySelf, err := util.DecodeUnpaddedBase64String(b64.Key)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, ed25519.PublicKey(keySelf), keyVal)
		}
	}
}
