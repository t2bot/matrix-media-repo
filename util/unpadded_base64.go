package util

import (
	"encoding/base64"
)

func DecodeUnpaddedBase64String(val string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(val)
}

func EncodeUnpaddedBase64ToString(val []byte) string {
	return base64.RawStdEncoding.EncodeToString(val)
}
