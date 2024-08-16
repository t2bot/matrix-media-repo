package synapse

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/internal"
	"github.com/t2bot/matrix-media-repo/util"
)

func EncodeSigningKey(key *homeserver_interop.SigningKey) ([]byte, error) {
	b64 := util.EncodeUnpaddedBase64ToString(key.PrivateKey.Seed())
	return []byte(fmt.Sprintf("ed25519 %s %s", key.KeyVersion, b64)), nil
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

	// See https://github.com/matrix-org/python-signedjson/blob/067ae81616573e8ceb627cc046d91b5b489bcc96/signedjson/key.py#L137-L150
	// See https://github.com/matrix-org/python-signedjson/blob/067ae81616573e8ceb627cc046d91b5b489bcc96/LICENSE
	lines := strings.Split(string(b), "\n")
	if len(lines) <= 0 {
		return nil, fmt.Errorf("no signing keys found")
	}
	keys := make([]*homeserver_interop.SigningKey, 0)
	for i, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 3 {
			return nil, fmt.Errorf("i:%d - expected 3 parts to signing key, got %d", i, len(parts))
		}

		if parts[0] != "ed25519" {
			return nil, fmt.Errorf("i:%d - expected ed25519 signing key, got '%s'", i, parts[0])
		}

		keyVersion := parts[1]
		keyB64 := parts[2]

		keyBytes, err := util.DecodeUnpaddedBase64String(keyB64)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("i:%d - expected base64 signing key part", i), err)
		}

		_, priv, err := ed25519.GenerateKey(bytes.NewReader(keyBytes))
		if err != nil {
			return nil, errors.Join(fmt.Errorf("i:%d - error generating ed25519 key", i), err)
		}

		keys = append(keys, &homeserver_interop.SigningKey{
			PrivateKey: priv,
			KeyVersion: keyVersion,
		})
	}
	return keys, nil
}
