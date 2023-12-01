package dendrite

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"io"
	"strings"

	"github.com/turt2live/matrix-media-repo/homeserver_interop"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/internal"
)

const blockType = "MATRIX PRIVATE KEY"

func EncodeSigningKey(key *homeserver_interop.SigningKey) ([]byte, error) {
	block := &pem.Block{
		Type: blockType,
		Headers: map[string]string{
			"Key-ID": fmt.Sprintf("ed25519:%s", key.KeyVersion),
		},
		Bytes: key.PrivateKey.Seed(),
	}
	return pem.EncodeToMemory(block), nil
}

func EncodeAllSigningKeys(keys []*homeserver_interop.SigningKey) ([]byte, error) {
	return internal.EncodeNewlineAppendFormattedSigningKeys(keys, EncodeSigningKey)
}

func DecodeSigningKey(key io.Reader) (*homeserver_interop.SigningKey, error) {
	keys, err := DecodeAllSigningKeys(key)
	if err != nil {
		return nil, err
	}

	return keys[0], nil
}

func DecodeAllSigningKeys(key io.Reader) ([]*homeserver_interop.SigningKey, error) {
	b, err := io.ReadAll(key)
	if err != nil {
		return nil, err
	}

	// See https://github.com/matrix-org/dendrite/blob/fd11e65a9d113839177f9f7a32af328a0292b195/setup/config/config.go#L446
	// See https://github.com/matrix-org/dendrite/blob/fd11e65a9d113839177f9f7a32af328a0292b195/LICENSE
	keys := make([]*homeserver_interop.SigningKey, 0)
	var block *pem.Block
	for {
		block, b = pem.Decode(b)
		if b == nil {
			return nil, fmt.Errorf("no signing key found")
		}
		if block == nil && (len(keys) == 0 || len(b) > 0) {
			return nil, fmt.Errorf("unable to read suitable block from PEM file")
		} else if block == nil && len(b) == 0 {
			break
		}
		if block.Type == blockType {
			keyId := block.Headers["Key-ID"]
			if len(keyId) <= 0 {
				return nil, fmt.Errorf("missing Key-ID header")
			}
			if !strings.HasPrefix(keyId, "ed25519:") {
				return nil, fmt.Errorf("key ID '%s' does not denote an ed25519 private key", keyId)
			}

			_, priv, err := ed25519.GenerateKey(bytes.NewReader(block.Bytes))
			if err != nil {
				return nil, err
			}

			keys = append(keys, &homeserver_interop.SigningKey{
				PrivateKey: priv,
				KeyVersion: keyId[len("ed25519:"):],
			})
		}
	}

	return keys, nil
}
