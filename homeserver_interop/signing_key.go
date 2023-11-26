package homeserver_interop

import (
	"crypto/ed25519"
)

type SigningKey struct {
	PrivateKey ed25519.PrivateKey
	KeyVersion string
}
