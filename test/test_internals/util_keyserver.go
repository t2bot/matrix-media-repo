package test_internals

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"log"

	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
	"github.com/t2bot/matrix-media-repo/util"
)

func MakeKeyServer(deps *ContainerDeps) (*HostedFile, *homeserver_interop.SigningKey) {
	// We'll use a pre-computed signing key for simplicity
	signingKey, err := mmr.DecodeSigningKey(bytes.NewReader([]byte(`-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:e5d0oC
Version: 1

PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgc=
-----END MMR PRIVATE KEY-----
`)))
	if err != nil {
		log.Fatal(err)
	}
	keyServerKey := signingKey
	// Create a /_matrix/key/v2/server response file (signed JSON)
	keyServer, writeFn, err := LazyServeFile("_matrix/key/v2/server", deps)
	if err != nil {
		log.Fatal(err)
	}
	serverKey := database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     keyServer.PublicHostname,
		"valid_until_ts":  util.NowMillis() + (60 * 60 * 1000), // +1hr
		"verify_keys": database.AnonymousJson{
			"ed25519:e5d0oC": database.AnonymousJson{
				"key": "TohekYXzLx7VzV8FtLQlI3XsSdPv1CjhVYY5rZmFCvU",
			},
		},
	}
	canonical, err := util.EncodeCanonicalJson(serverKey)
	signature := util.EncodeUnpaddedBase64ToString(ed25519.Sign(signingKey.PrivateKey, canonical))
	serverKey["signatures"] = database.AnonymousJson{
		keyServer.PublicHostname: database.AnonymousJson{
			"ed25519:e5d0oC": signature,
		},
	}
	b, err := json.Marshal(serverKey)
	if err != nil {
		log.Fatal(err)
	}
	err = writeFn(string(b))
	if err != nil {
		log.Fatal(err)
	}

	return keyServer, keyServerKey
}
