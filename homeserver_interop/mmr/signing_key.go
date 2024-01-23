package mmr

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"io"
	"strings"

	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/internal"
)

const blockType = "MMR PRIVATE KEY"

func EncodeSigningKey(key *homeserver_interop.SigningKey) ([]byte, error) {
	// Similar to Dendrite, but using a different block type and added Version header for future expansion
	block := &pem.Block{
		Type: blockType,
		Headers: map[string]string{
			"Key-ID":  fmt.Sprintf("ed25519:%s", key.KeyVersion),
			"Version": "1",
		},
		Bytes: key.PrivateKey.Seed(),
	}
	b := pem.EncodeToMemory(block)
	return b, nil
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
			version := block.Headers["Version"]
			if version != "1" {
				return nil, fmt.Errorf("unsupported MMR key format version")
			}

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
