package cdn

import (
	"context"
	"errors"

	"github.com/fastly/fastly-go/fastly"
)

type FastlyCdn struct {
	cli            *fastly.APIClient
	ctx            context.Context
	serviceId      string
	dictionaryName string
}

func NewFastlyCdn(apiKey string, serviceId string, dictionaryName string) *FastlyCdn {
	if apiKey == "" || serviceId == "" || dictionaryName == "" {
		return nil
	}

	client := fastly.NewAPIClient(fastly.NewConfiguration())
	ctx := fastly.NewAPIKeyContext(apiKey)
	return &FastlyCdn{
		cli:            client,
		ctx:            ctx,
		serviceId:      serviceId,
		dictionaryName: dictionaryName,
	}
}

func (f *FastlyCdn) SetDictionaryItem(key string, value string) error {
	// Find the latest service version
	versions, _, err := f.cli.VersionAPI.ListServiceVersions(f.ctx, f.serviceId).Execute()
	if err != nil {
		return err
	}
	var latestVersion *fastly.VersionResponse
	for _, v := range versions {
		if v.GetActive() && (latestVersion == nil || latestVersion.GetNumber() < v.GetNumber()) {
			latestVersion = &v
		}
	}
	if latestVersion == nil {
		return errors.New("no active service versions to configure")
	}

	// Find and update the dictionary
	d, _, err := f.cli.DictionaryAPI.GetDictionary(f.ctx, f.serviceId, latestVersion.GetNumber(), f.dictionaryName).Execute()
	if err != nil {
		return err
	}
	req := f.cli.DictionaryItemAPI.UpsertDictionaryItem(f.ctx, f.serviceId, *d.ID, key)
	req.ItemValue(value)
	_, _, err = req.Execute()
	return err
}
