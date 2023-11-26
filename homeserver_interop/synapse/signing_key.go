package synapse

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/turt2live/matrix-media-repo/util"
)

func EncodeSigningKey(keyVersion string, key ed25519.PrivateKey) ([]byte, error) {
	b64 := util.EncodeUnpaddedBase64ToString(key.Seed())
	return []byte(fmt.Sprintf("ed25519 %s %s", keyVersion, b64)), nil
}

func DecodeSigningKey(key io.Reader) (ed25519.PrivateKey, string, error) {
	b, err := io.ReadAll(key)
	if err != nil {
		return nil, "", err
	}

	// See https://github.com/matrix-org/python-signedjson/blob/067ae81616573e8ceb627cc046d91b5b489bcc96/signedjson/key.py#L137-L150
	parts := strings.Split(string(b), " ")
	if len(parts) != 3 {
		return nil, "", fmt.Errorf("expected 3 parts to signing key, got %d", len(parts))
	}

	if parts[0] != "ed25519" {
		return nil, "", fmt.Errorf("expected ed25519 signing key, got '%s'", parts[0])
	}

	keyVersion := parts[1]
	keyB64 := parts[2]

	keyBytes, err := util.DecodeUnpaddedBase64String(keyB64)
	if err != nil {
		return nil, "", errors.Join(errors.New("expected base64 signing key part"), err)
	}

	_, priv, err := ed25519.GenerateKey(bytes.NewReader(keyBytes))
	if err != nil {
		return nil, "", err
	}

	return priv, keyVersion, nil
}
