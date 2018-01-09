package util

import "github.com/turt2live/matrix-media-repo/config"

func IsServerOurs(server string) (bool) {
	hs := GetHomeserverConfig(server)
	return hs != nil
}

func GetHomeserverConfig(server string) (*config.HomeserverConfig) {
	for i := 0; i < len(config.Get().Homeservers); i++ {
		hs := config.Get().Homeservers[i]
		if hs.Name == server {
			return &hs
		}
	}

	return nil
}
