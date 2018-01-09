package rcontext

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage"
)

type RequestInfo struct {
	Context context.Context
	Log     *logrus.Entry
	Db      storage.Database
}
