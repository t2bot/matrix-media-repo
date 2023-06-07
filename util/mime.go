package util

import (
	"mime"
	"strings"
)

func FixContentType(ct string) string {
	return strings.Split(ct, ";")[0]
}

func ExtensionForContentType(ct string) string {
	exts, _ := mime.ExtensionsByType(ct)
	if exts != nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}
