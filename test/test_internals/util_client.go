package test_internals

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

type MatrixClient struct {
	AccessToken        string
	ClientServerUrl    string
	UserId             string
	ServerName         string
	AuthHeaderOverride string
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
	val := new(MatrixUploadResponse)
	err := c.DoReturnJson("POST", "/_matrix/media/v3/upload", url.Values{"filename": []string{filename}}, contentType, body, val)
	return val, err
}

func (c *MatrixClient) DoReturnJson(method string, endpoint string, qs url.Values, contentType string, body io.Reader, retVal interface{}) error {
	res, err := c.DoRaw(method, endpoint, qs, contentType, body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("%d : %s", res.StatusCode, string(b))
	}

	decoder := json.NewDecoder(res.Body)
	return decoder.Decode(&retVal)
}

func (c *MatrixClient) DoExpectError(method string, endpoint string, qs url.Values, contentType string, body io.Reader) (*MatrixErrorResponse, error) {
	res, err := c.DoRaw(method, endpoint, qs, contentType, body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusOK {
		return nil, errors.New("expected non-200 status code")
	}

	decoder := json.NewDecoder(res.Body)
	retVal := new(MatrixErrorResponse)
	err = decoder.Decode(&retVal)
	retVal.InjectedStatusCode = res.StatusCode
	return retVal, err
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
	if c.ServerName != "" {
		req.Host = c.ServerName
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	if c.AuthHeaderOverride != "" {
		req.Header.Set("Authorization", c.AuthHeaderOverride)
	}

	log.Printf("[HTTP] [Auth=%s] [Host=%s] %s %s", req.Header.Get("Authorization"), c.ServerName, req.Method, req.URL.String())
	return http.DefaultClient.Do(req)
}
