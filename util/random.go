package util

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
)

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func GenerateRandomString(nBytes int) (string, error) {
	b, err := GenerateRandomBytes(nBytes)
	if err != nil {
		return "", err
	}

	hasher := sha1.New()
	hasher.Write(b)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
