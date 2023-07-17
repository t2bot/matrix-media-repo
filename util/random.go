package util

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
)

func GenerateRandomString(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	hasher := sha1.New()
	hasher.Write(b)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
