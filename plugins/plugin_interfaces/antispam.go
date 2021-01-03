package plugin_interfaces

import (
	"encoding/json"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

type Antispam interface {
	HandleConfig(config map[string]interface{}) error
	CheckForSpam(b64 string, filename string, contentType string, userId string, origin string, mediaId string) (bool, error)
}

type AntispamRPC struct {
	client *rpc.Client
}

func (g *AntispamRPC) HandleConfig(config map[string]interface{}) error {
	var i string
	b, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return g.client.Call("Plugin.HandleConfig", map[string]interface{}{"c": string(b)}, &i)
}

func (g *AntispamRPC) CheckForSpam(b64 string, filename string, contentType string, userId string, origin string, mediaId string) (bool, error) {
	var resp bool
	err := g.client.Call("Plugin.CheckForSpam", map[string]interface{}{
		"b64":         b64,
		"filename":    filename,
		"contentType": contentType,
		"userId":      userId,
		"origin":      origin,
		"mediaId":     mediaId,
	}, &resp)
	return resp, err
}

type AntispamRPCServer struct {
	Impl Antispam
}

func (s *AntispamRPCServer) HandleConfig(args map[string]interface{}, resp *string) error {
	*resp = "not_used"
	var conf map[string]interface{}
	err := json.Unmarshal(([]byte)(args["c"].(string)), &conf)
	if err != nil {
		return err
	}
	return s.Impl.HandleConfig(conf)
}

func (s *AntispamRPCServer) CheckForSpam(args map[string]interface{}, resp *bool) error {
	var err error
	*resp, err = s.Impl.CheckForSpam(
		args["b64"].(string),
		args["filename"].(string),
		args["contentType"].(string),
		args["userId"].(string),
		args["origin"].(string),
		args["mediaId"].(string),
	)
	return err
}

type AntispamPlugin struct {
	Impl Antispam
}

func (p *AntispamPlugin) Server(broker *plugin.MuxBroker) (interface{}, error) {
	return &AntispamRPCServer{Impl: p.Impl}, nil
}

func (p *AntispamPlugin) Client(broker *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &AntispamRPC{client: c}, nil
}
