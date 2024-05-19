package util

import (
	"mime"
	"strings"
)

func FixContentType(ct string) string {
	return strings.Split(ct, ";")[0]
}

func ExtensionForContentType(ct string) string {
	// custom overrides
	if ct == "image/png" {
		return ".png"
	} else if ct == "image/jpeg" {
		return ".jpg"
	}

	// Otherwise look it up
	exts, _ := mime.ExtensionsByType(ct)
	if len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func CanInline(ct string) bool {
	ct = FixContentType(ct)
	return ArrayContains(InlineContentTypes, ct)
}

var InlineContentTypes = []string{
	// Types are inherited from https://github.com/matrix-org/synapse/pull/15988

	"text/css",
	"text/plain",
	"text/csv",
	"application/json",
	"application/ld+json",
	"image/jpeg",
	"image/gif",
	"image/png",
	"image/apng",
	"image/webp",
	"image/avif",
	"video/mp4",
	"video/webm",
	"video/ogg",
	"video/quicktime",
	"audio/mp4",
	"audio/webm",
	"audio/aac",
	"audio/mpeg",
	"audio/ogg",
	"audio/wave",
	"audio/wav",
	"audio/x-wav",
	"audio/x-pn-wav",
	"audio/flac",
	"audio/x-flac",
}
