package util

import "github.com/turt2live/matrix-media-repo/types"

func MediaToMxc(media *types.Media) string {
	return "mxc://" + media.Origin + "/" + media.MediaId
}
