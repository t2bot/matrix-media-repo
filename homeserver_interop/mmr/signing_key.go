package mmr

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"io"
	"strings"
)

const blockType = "MMR PRIVATE KEY"

func EncodeSigningKey(keyVersion string, key ed25519.PrivateKey) ([]byte, error) {
	// Similar to Dendrite, but using a different block type and added Version header for future expansion
	block := &pem.Block{
		Type: blockType,
		Headers: map[string]string{
			"Key-ID":  fmt.Sprintf("ed25519:%s", keyVersion),
			"Version": "1",
		},
		Bytes: key.Seed(),
	}
	return pem.EncodeToMemory(block), nil
}

func DecodeSigningKey(key io.Reader) (ed25519.PrivateKey, string, error) {
	b, err := io.ReadAll(key)
	if err != nil {
		return nil, "", err
	}

	var block *pem.Block
	for {
		block, b = pem.Decode(b)
		if b == nil {
			return nil, "", fmt.Errorf("no signing key found")
		}
		if block == nil {
			return nil, "", fmt.Errorf("unable to read suitable block from PEM file")
		}
		if block.Type == blockType {
			version := block.Headers["Version"]
			if version != "1" {
				return nil, "", fmt.Errorf("unsupported MMR key format version")
			}

			keyId := block.Headers["Key-ID"]
			if len(keyId) <= 0 {
				return nil, "", fmt.Errorf("missing Key-ID header")
			}
			if !strings.HasPrefix(keyId, "ed25519:") {
				return nil, "", fmt.Errorf("key ID '%s' does not denote an ed25519 private key", keyId)
			}
			_, priv, err := ed25519.GenerateKey(bytes.NewReader(block.Bytes))
			if err != nil {
				return nil, "", err
			}

			return priv, keyId[len("ed25519:"):], nil
		}
	}
}
