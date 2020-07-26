package util

import (
	"strings"
)

func FixContentType(ct string) string {
	return strings.Split(ct, ";")[0]
}
