package util

import (
	"bytes"
	"encoding/json"
)

func EncodeCanonicalJson(obj any) ([]byte, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	// De-encode values
	b = bytes.Replace(b, []byte("\\u003c"), []byte("<"), -1)
	b = bytes.Replace(b, []byte("\\u003e"), []byte(">"), -1)
	b = bytes.Replace(b, []byte("\\u0026"), []byte("&"), -1)

	return b, nil
}
