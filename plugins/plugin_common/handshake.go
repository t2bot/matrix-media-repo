package plugin_common

import (
	"github.com/hashicorp/go-plugin"
)

// dev note: HandshakeConfig is for UX, not security

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "MEDIA_REPO_PLUGIN",
	MagicCookieValue: "hello world - I am a media repo",
}
