package plugins

import (
	"encoding/base64"
	"io"

	"github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/plugins/plugin_interfaces"
)

var pluginTypes = map[string]plugin.Plugin{
	"antispam": &plugin_interfaces.AntispamPlugin{},
}

var existingPlugins = make([]*mmrPlugin, 0)

func ReloadPlugins() {
	for _, pl := range config.Get().Plugins {
		logrus.Info("Loading plugin: ", pl.Executable)
		mmr, err := newPlugin(pl.Executable, pl.Config)
		if err != nil {
			logrus.Errorf("failed to load plugin %s: %s", pl.Executable, err.Error())
			continue
		}

		existingPlugins = append(existingPlugins, mmr)
	}
}

func StopPlugins() {
	if len(existingPlugins) == 0 {
		return
	}

	logrus.Info("Stopping plugin instances...")
	for _, pl := range existingPlugins {
		pl.Stop()
	}
	existingPlugins = make([]*mmrPlugin, 0)
}

func CheckForSpam(r io.Reader, filename string, contentType string, userId string, origin string, mediaId string) (bool, error) {
	b := make([]byte, 0)
	for _, pl := range existingPlugins {
		as, err := pl.Antispam()
		if err != nil {
			logrus.Warnf("error loading antispam plugin: %s", err.Error())
			continue
		}

		if len(b) == 0 {
			b, err = io.ReadAll(r)
			if err != nil {
				return false, err
			}
		}

		b64 := base64.StdEncoding.EncodeToString(b)
		spam, err := as.CheckForSpam(b64, filename, contentType, userId, origin, mediaId)
		if err != nil {
			return false, err
		}
		if spam {
			return true, err
		}
	}
	return false, nil
}
