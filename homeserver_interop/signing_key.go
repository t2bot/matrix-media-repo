package homeserver_interop

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
)

type SigningKey struct {
	PrivateKey ed25519.PrivateKey
	KeyVersion string
}

func GenerateSigningKey() (*SigningKey, error) {
	keyVersion := makeKeyVersion()

	_, priv, err := ed25519.GenerateKey(nil)
	priv = priv[len(priv)-32:]
	if err != nil {
		return nil, err
	}

	return &SigningKey{
		PrivateKey: priv,
		KeyVersion: keyVersion,
	}, nil
}

func makeKeyVersion() string {
	buf := make([]byte, 2)
	chars := strings.Split("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "")
	for i := 0; i < len(chars); i++ {
		sort.Slice(chars, func(i int, j int) bool {
			c, err := rand.Read(buf)

			// "should never happen" clauses
			if err != nil {
				panic(err)
			}
			if c != len(buf) || c != 2 {
				panic(fmt.Errorf("crypto rand read %d bytes, expected %d", c, len(buf)))
			}

			return buf[0] < buf[1]
		})
	}

	return strings.Join(chars[:6], "")
}
