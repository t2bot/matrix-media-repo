package matrix

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/go-typed-singleflight"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util"
)

type signingKey struct {
	Key string `json:"key"`
}

type serverKeyResult struct {
	ServerName    string                       `json:"server_name"`
	ValidUntilTs  int64                        `json:"valid_until_ts"`
	VerifyKeys    map[string]signingKey        `json:"verify_keys"`     // unpadded base64
	OldVerifyKeys map[string]signingKey        `json:"old_verify_keys"` // unpadded base64
	Signatures    map[string]map[string]string `json:"signatures"`      // unpadded base64; <name, <keyId, sig>>
}

type ServerSigningKeys map[string]ed25519.PublicKey

var signingKeySf = new(typedsf.Group[*ServerSigningKeys])
var signingKeyCache = cache.New(cache.NoExpiration, 30*time.Second)
var signingKeyRWLock = new(sync.RWMutex)

func querySigningKeyCache(serverName string) *ServerSigningKeys {
	if val, ok := signingKeyCache.Get(serverName); ok {
		return val.(*ServerSigningKeys)
	}
	return nil
}

func QuerySigningKeys(serverName string) (*ServerSigningKeys, error) {
	signingKeyRWLock.RLock()
	keys := querySigningKeyCache(serverName)
	signingKeyRWLock.RUnlock()
	if keys != nil {
		return keys, nil
	}

	keys, err, _ := signingKeySf.Do(serverName, func() (*ServerSigningKeys, error) {
		ctx := rcontext.Initial().LogWithFields(logrus.Fields{
			"keysForServer": serverName,
		})

		signingKeyRWLock.Lock()
		defer signingKeyRWLock.Unlock()

		// check cache once more, just in case the locks overlapped
		cachedKeys := querySigningKeyCache(serverName)
		if keys != nil {
			return cachedKeys, nil
		}

		// now we can try to get the keys from the source
		url, hostname, err := GetServerApiUrl(serverName)
		if err != nil {
			return nil, err
		}

		keysUrl := url + "/_matrix/key/v2/server"
		keysResponse, err := FederatedGet(keysUrl, hostname, ctx)
		if keysResponse != nil {
			defer keysResponse.Body.Close()
		}
		if err != nil {
			return nil, err
		}

		decoder := json.NewDecoder(keysResponse.Body)
		raw := database.AnonymousJson{}
		if err = decoder.Decode(&raw); err != nil {
			return nil, err
		}
		keyInfo := new(serverKeyResult)
		if err = raw.ApplyTo(keyInfo); err != nil {
			return nil, err
		}

		// Check validity before we go much further
		if keyInfo.ServerName != serverName {
			return nil, fmt.Errorf("got keys for '%s' but expected '%s'", keyInfo.ServerName, serverName)
		}
		if keyInfo.ValidUntilTs <= util.NowMillis() {
			return nil, errors.New("returned server keys are expired")
		}
		cacheUntil := time.Until(time.UnixMilli(keyInfo.ValidUntilTs)) / 2
		if cacheUntil <= (6 * time.Second) {
			return nil, errors.New("returned server keys would expire too quickly")
		}

		// Convert to something useful
		serverKeys := make(ServerSigningKeys)
		for keyId, keyObj := range keyInfo.VerifyKeys {
			b, err := util.DecodeUnpaddedBase64String(keyObj.Key)
			if err != nil {
				return nil, errors.Join(fmt.Errorf("bad base64 for key ID '%s' for '%s'", keyId, serverName), err)
			}

			serverKeys[keyId] = b
		}

		// Check signatures
		if len(keyInfo.Signatures) == 0 || len(keyInfo.Signatures[serverName]) == 0 {
			return nil, fmt.Errorf("missing signatures from '%s'", serverName)
		}
		delete(raw, "signatures")
		canonical, err := util.EncodeCanonicalJson(raw)
		if err != nil {
			return nil, err
		}
		for domain, sig := range keyInfo.Signatures {
			if domain != serverName {
				return nil, fmt.Errorf("unexpected signature from '%s' (expected '%s')", domain, serverName)
			}

			for keyId, b64 := range sig {
				signatureBytes, err := util.DecodeUnpaddedBase64String(b64)
				if err != nil {
					return nil, errors.Join(fmt.Errorf("bad base64 signature for key ID '%s' for '%s'", keyId, serverName), err)
				}

				key, ok := serverKeys[keyId]
				if !ok {
					return nil, fmt.Errorf("unknown key ID '%s' for signature from '%s'", keyId, serverName)
				}

				if !ed25519.Verify(key, canonical, signatureBytes) {
					return nil, fmt.Errorf("invalid signature '%s' from key ID '%s' for '%s'", b64, keyId, serverName)
				}
			}
		}

		// Cache & return (unlock was deferred)
		signingKeyCache.Set(serverName, &serverKeys, cacheUntil)
		return &serverKeys, nil
	})
	return keys, err
}
