package matrix

import (
	"crypto/ed25519"
	"os"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
)

type LocalSigningKey struct {
	Key     ed25519.PrivateKey
	Version string
}

var localSigningKeyCache = cache.New(5*time.Minute, 10*time.Minute)

func FlushSigningKeyCache() {
	localSigningKeyCache.Flush()
}

func getLocalSigningKey(fromPath string) (*LocalSigningKey, error) {
	if val, ok := localSigningKeyCache.Get(fromPath); ok {
		return val.(*LocalSigningKey), nil
	}

	f, err := os.Open(fromPath)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	key, err := mmr.DecodeSigningKey(f)
	if err != nil {
		return nil, err
	}
	sk := &LocalSigningKey{
		Key:     key.PrivateKey,
		Version: key.KeyVersion,
	}
	localSigningKeyCache.Set(fromPath, sk, cache.DefaultExpiration)
	return sk, nil
}
