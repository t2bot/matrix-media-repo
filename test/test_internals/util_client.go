package test_internals

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/turt2live/matrix-media-repo/database"
)

type MatrixClient struct {
	AccessToken     string
	ClientServerUrl string
	UserId          string
	ServerName      string
}

func (c *MatrixClient) WithCsUrl(newUrl string) *MatrixClient {
	return &MatrixClient{
		AccessToken:     c.AccessToken,
		ClientServerUrl: newUrl,
		UserId:          c.UserId,
		ServerName:      c.ServerName,
	}
}

func (c *MatrixClient) Upload(filename string, contentType string, body io.Reader) (*MatrixUploadResponse, error) {
	j, err := c.DoReturnJson("POST", "/_matrix/media/v3/upload", url.Values{"filename": []string{filename}}, contentType, body)
	if err != nil {
		return nil, err
	}
	val := new(MatrixUploadResponse)
	err = j.ApplyTo(&val)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (c *MatrixClient) DoReturnJson(method string, endpoint string, qs url.Values, contentType string, body io.Reader) (*database.AnonymousJson, error) {
	res, err := c.DoRaw(method, endpoint, qs, contentType, body)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(res.Body)
	val := new(database.AnonymousJson)
	err = decoder.Decode(&val)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (c *MatrixClient) DoRaw(method string, endpoint string, qs url.Values, contentType string, body io.Reader) (*http.Response, error) {
	endpoint, err := url.JoinPath(c.ClientServerUrl, endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, endpoint+"?"+qs.Encode(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}

	log.Println(fmt.Sprintf("[HTTP] [Auth=%s] %s %s", c.AccessToken, req.Method, req.RequestURI))
	return http.DefaultClient.Do(req)
}
