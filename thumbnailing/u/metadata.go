package u

import (
	"bytes"

	"github.com/dhowden/tag"
)

func GetID3Tags(b []byte) tag.Metadata {
	meta, _ := tag.ReadFrom(bytes.NewReader(b))
	return meta
}
