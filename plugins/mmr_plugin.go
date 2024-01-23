package plugins

import (
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/plugins/plugin_common"
	"github.com/t2bot/matrix-media-repo/plugins/plugin_interfaces"
)

type mmrPlugin struct {
	hcClient  *plugin.Client
	rpcClient plugin.ClientProtocol
	config    map[string]interface{}

	antispamPlugin plugin_interfaces.Antispam
}

func newPlugin(path string, config map[string]interface{}) (*mmrPlugin, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: logrus.WithField("plugin", path).Writer(),
		Level:  hclog.Debug,
	})
	client := plugin.NewClient(&plugin.ClientConfig{
		Cmd:             exec.Command(path),
		Logger:          logger,
		HandshakeConfig: plugin_common.HandshakeConfig,
		Plugins:         pluginTypes,
	})
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, err
	}
	return &mmrPlugin{
		hcClient:  client,
		rpcClient: rpcClient,
		config:    config,
	}, nil
}

func (p *mmrPlugin) Antispam() (plugin_interfaces.Antispam, error) {
	if p.antispamPlugin != nil {
		return p.antispamPlugin, nil
	}

	raw, err := p.rpcClient.Dispense("antispam")
	if err != nil {
		return nil, err
	}

	p.antispamPlugin = raw.(plugin_interfaces.Antispam)
	_ = p.antispamPlugin.HandleConfig(p.config)
	return p.antispamPlugin, nil
}

func (p *mmrPlugin) Stop() {
	p.antispamPlugin = nil
	p.hcClient.Kill()
}
