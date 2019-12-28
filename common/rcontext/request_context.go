package rcontext

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
)

func Initial() RequestContext {
	return RequestContext{
		Context: context.Background(),
		Log:     logrus.WithFields(logrus.Fields{"nocontext": true}),
		Config: config.DomainRepoConfig{
			MinimumRepoConfig: config.Get().MinimumRepoConfig,
			Downloads:         config.Get().Downloads.DownloadsConfig,
			Thumbnails:        config.Get().Thumbnails.ThumbnailsConfig,
			UrlPreviews:       config.Get().UrlPreviews.UrlPreviewsConfig,
		},
	}.populate()
}

type RequestContext struct {
	context.Context

	// These are also stored on the context object itself
	Log    *logrus.Entry           // mr.logger
	Config config.DomainRepoConfig // mr.serverConfig
}

func (c RequestContext) populate() RequestContext {
	c.Context = context.WithValue(c.Context, "mr.logger", c.Log)
	//c.Context = context.WithValue(c.Context, "mr.serverConfig", c.Config)
	return c
}

func (c RequestContext) ReplaceLogger(log *logrus.Entry) RequestContext {
	ctx := context.WithValue(c.Context, "mr.logger", log)
	return RequestContext{
		Context: ctx,
		Log:     log,
		Config:  c.Config,
	}
}

func (c RequestContext) LogWithFields(fields logrus.Fields) RequestContext {
	return c.ReplaceLogger(c.Log.WithFields(fields))
}
