package _common

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/logging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
)

type MediaMetadata struct {
	MediaId        string
	ContentType    string
	FileName       string
	UploaderUserId string
	SizeBytes      int64
}

func PsqlMatrixDownloadCopy[M homeserver_interop.ImportDbMedia](ctx rcontext.RequestContext, cfg *ImportOptsPsqlFlatFile, db homeserver_interop.ImportDb[M], extractFn func(record *M) (*MediaMetadata, error)) {
	ctx.Log.Debug("Fetching all local media records from homeserver...")
	records, err := db.GetAllMedia()
	if err != nil {
		panic(err)
	}

	ctx.Log.Info(fmt.Sprintf("Downloading %d media records", len(records)))

	pool, err := ants.NewPool(cfg.NumWorkers, ants.WithOptions(ants.Options{
		ExpiryDuration:   1 * time.Hour,
		PreAlloc:         false,
		MaxBlockingTasks: 0, // no limit
		Nonblocking:      false,
		Logger:           &logging.SendToDebugLogger{},
		DisablePurge:     false,
		PanicHandler: func(err interface{}) {
			panic(err)
		},
	}))
	if err != nil {
		panic(err)
	}

	numCompleted := 0
	mu := &sync.RWMutex{}
	onComplete := func() {
		mu.Lock()
		numCompleted++
		percent := int((float32(numCompleted) / float32(len(records))) * 100)
		ctx.Log.Info(fmt.Sprintf("%d/%d downloaded (%d%%)", numCompleted, len(records), percent))
		mu.Unlock()
	}

	for i := 0; i < len(records); i++ {
		percent := int((float32(i+1) / float32(len(records))) * 100)
		record := records[i]

		meta, err := extractFn(record)
		if err != nil {
			panic(err)
		}

		ctx.Log.Debug(fmt.Sprintf("Queuing %s (%d/%d %d%%)", meta.MediaId, i+1, len(records), percent))
		err = pool.Submit(doWork(ctx, meta, cfg.ServerName, cfg.ApiUrl, onComplete))
		if err != nil {
			panic(err)
		}
	}

	for numCompleted < len(records) {
		ctx.Log.Debug("Waiting for import to complete...")
		time.Sleep(1 * time.Second)
	}

	ctx.Log.Info("Import completed")
}

func doWork(ctx rcontext.RequestContext, record *MediaMetadata, serverName string, csApiUrl string, onComplete func()) func() {
	return func() {
		defer onComplete()

		ctx := ctx.LogWithFields(logrus.Fields{"origin": serverName, "mediaId": record.MediaId})

		db := database.GetInstance().Media.Prepare(ctx)

		dbRecord, err := db.GetById(serverName, record.MediaId)
		if err != nil {
			panic(err)
		}
		if dbRecord != nil {
			ctx.Log.Debug("Already downloaded - skipping")
			return
		}

		body, err := downloadMedia(csApiUrl, serverName, record.MediaId)
		if err != nil {
			panic(err)
		}

		dbRecord, err = pipeline_upload.Execute(ctx, serverName, record.MediaId, body, record.ContentType, record.FileName, record.UploaderUserId, datastores.LocalMediaKind)
		if err != nil {
			panic(err)
		}

		if dbRecord.SizeBytes != record.SizeBytes {
			ctx.Log.Warnf("Size mismatch! Expected %d bytes but got %d", record.SizeBytes, dbRecord.SizeBytes)
		}
	}
}

func downloadMedia(baseUrl string, serverName string, mediaId string) (io.ReadCloser, error) {
	downloadUrl := baseUrl + "/_matrix/media/v3/download/" + serverName + "/" + mediaId
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("received status code " + strconv.Itoa(resp.StatusCode))
	}

	return resp.Body, nil
}
