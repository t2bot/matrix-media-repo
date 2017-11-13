package util

import (
	"crypto/md5"
	"crypto/rand"
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

	hasher := md5.New()
	hasher.Write(b)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
