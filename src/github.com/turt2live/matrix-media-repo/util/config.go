package util

import "github.com/turt2live/matrix-media-repo/config"

func IsServerOurs(server string, c config.MediaRepoConfig) (bool) {
	hs := GetHomeserverConfig(server, c)
	return hs != nil
}

func GetHomeserverConfig(server string, c config.MediaRepoConfig) (*config.HomeserverConfig) {
	for i := 0; i < len(c.Homeservers); i++ {
		hs := c.Homeservers[i]
		if hs.Name == server {
			return &hs
		}
	}

	return nil
}
