package util

import "github.com/turt2live/matrix-media-repo/config"

func IsServerOurs(server string, c config.MediaRepoConfig) (bool) {
	for i := 0; i < len(c.Homeservers); i++ {
		hs := c.Homeservers[i]
		if hs.Name == server {
			return true
		}
	}

	return false
}
