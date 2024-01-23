package internal

import (
	"bytes"
	"fmt"

	"github.com/t2bot/matrix-media-repo/homeserver_interop"
)

func EncodeNewlineAppendFormattedSigningKeys(keys []*homeserver_interop.SigningKey, encodeFn func(*homeserver_interop.SigningKey) ([]byte, error)) ([]byte, error) {
	buf := &bytes.Buffer{}
	for i, key := range keys {
		b, err := encodeFn(key)
		if err != nil {
			return nil, err
		}

		n, err := buf.Write(b)
		if err != nil {
			return nil, err
		}
		if n != len(b) {
			return nil, fmt.Errorf("wrote %d bytes but expected %d bytes", n, len(b))
		}

		if b[len(b)-1] != '\n' && i != (len(keys)-1) {
			n, err = buf.Write([]byte{'\n'})
			if err != nil {
				return nil, err
			}
			if n != 1 {
				return nil, fmt.Errorf("wrote %d bytes but expected %d bytes", n, 1)
			}
		}
	}
	return buf.Bytes(), nil
}
