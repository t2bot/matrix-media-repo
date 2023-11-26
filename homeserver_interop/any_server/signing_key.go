package any_server

import (
	"crypto/ed25519"
	"errors"
	"io"

	"github.com/turt2live/matrix-media-repo/homeserver_interop/dendrite"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/mmr"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
)

func DecodeSigningKey(key io.ReadSeeker) (ed25519.PrivateKey, string, error) {
	var keyVersion string
	var priv ed25519.PrivateKey
	var err error

	var errorStack error

	// Try Synapse first, as the most popular
	priv, keyVersion, err = synapse.DecodeSigningKey(key)
	if err == nil {
		return priv, keyVersion, nil
	}
	errorStack = errors.Join(errors.New("synapse: unable to decode"), err, errorStack)

	// Rewind & try Dendrite
	if _, err = key.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}
	priv, keyVersion, err = dendrite.DecodeSigningKey(key)
	if err == nil {
		return priv, keyVersion, nil
	}
	errorStack = errors.Join(errors.New("dendrite: unable to decode"), err, errorStack)

	// Rewind & try MMR
	if _, err = key.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}
	priv, keyVersion, err = mmr.DecodeSigningKey(key)
	if err == nil {
		return priv, keyVersion, nil
	}
	errorStack = errors.Join(errors.New("mmr: unable to decode"), err, errorStack)

	// Fail case
	return nil, "", errors.Join(errors.New("unable to detect signing key format"), errorStack)
}
