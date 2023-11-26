package any_server

import (
	"errors"
	"io"

	"github.com/turt2live/matrix-media-repo/homeserver_interop"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/dendrite"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/mmr"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
)

func DecodeSigningKey(key io.ReadSeeker) (*homeserver_interop.SigningKey, error) {
	keys, err := DecodeAllSigningKeys(key)
	if err != nil {
		return nil, err
	}

	return keys[0], nil
}

func DecodeAllSigningKeys(key io.ReadSeeker) ([]*homeserver_interop.SigningKey, error) {
	var keys []*homeserver_interop.SigningKey
	var err error

	var errorStack error

	// Try Synapse first, as the most popular
	keys, err = synapse.DecodeAllSigningKeys(key)
	if err == nil {
		return keys, nil
	}
	errorStack = errors.Join(errors.New("synapse: unable to decode"), err, errorStack)

	// Rewind & try Dendrite
	if _, err = key.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	keys, err = dendrite.DecodeAllSigningKeys(key)
	if err == nil {
		return keys, nil
	}
	errorStack = errors.Join(errors.New("dendrite: unable to decode"), err, errorStack)

	// Rewind & try MMR
	if _, err = key.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	keys, err = mmr.DecodeAllSigningKeys(key)
	if err == nil {
		return keys, nil
	}
	errorStack = errors.Join(errors.New("mmr: unable to decode"), err, errorStack)

	// Fail case
	return nil, errors.Join(errors.New("unable to detect signing key format"), errorStack)
}
