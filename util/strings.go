package util

import (
	"strings"
)

func HasAnyPrefix(val string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(val, p) {
			return true
		}
	}
	return false
}
