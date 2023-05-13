package ids

import (
	"github.com/turt2live/matrix-media-repo/util"
)

func NewUniqueId() (string, error) {
	r, err := util.GenerateRandomString(32) // pad out the snowflake
	if err != nil {
		return "", err
	}
	sf, err := makeSnowflake()
	if err != nil {
		return "", err
	}
	return r + sf.Generate().String(), nil
}
