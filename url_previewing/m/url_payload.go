package m

import (
	"net/url"
)

type UrlPayload struct {
	UrlString string
	ParsedUrl *url.URL
}
