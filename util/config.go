package util

import (
	"strings"

	"github.com/turt2live/matrix-media-repo/common/config"
)

func IsServerOurs(server string) bool {
	hs := config.GetDomain(server)
	return hs != nil
}

func IsGlobalAdmin(userId string) bool {
	for _, admin := range config.Get().Admins {
		if admin == userId {
			return true
		}
	}

	return false
}

func IsHostIgnored(serverName string) bool {
	serverName = strings.ToLower(serverName)
	for _, host := range config.Get().Federation.IgnoredHosts {
		if strings.ToLower(host) == serverName {
			return true
		}
	}
	return false
}
