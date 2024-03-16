package util

import (
	"strings"
)

func HasAnyPrefix(val string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(val, prefix) {
			return true
		}
	}
	return false
}
