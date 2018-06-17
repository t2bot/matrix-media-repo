package types

import (
	"crypto/cipher"
	"encoding/base64"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blowfish"
)

type BearerToken struct {
	EncryptedToken   string
	AppserviceUserId string
	Host             string
}

func (b *BearerToken) RetrieveAccessToken(rawContentToken string) (string, error) {
	if b.EncryptedToken == "" {
		return "", errors.New("no encrypted token to decrypt")
	}

	decoded, err := base64.StdEncoding.DecodeString(b.EncryptedToken)
	if err != nil {
		return "", err
	}

	bfCipher, err := blowfish.NewCipher([]byte(rawContentToken))
	if err != nil {
		return "", err
	}

	if len(decoded) < blowfish.BlockSize {
		return "", errors.New("payload not long enough")
	}

	iv := decoded[:blowfish.BlockSize]
	decrypted := decoded[blowfish.BlockSize:]

	if len(decrypted)%blowfish.BlockSize != 0 {
		return "", errors.New("block size mismatch")
	}

	cbc := cipher.NewCBCDecrypter(bfCipher, iv)
	cbc.CryptBlocks(decrypted, decrypted)
	return string(decrypted), nil
}
