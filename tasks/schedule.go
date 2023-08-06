package tasks

import (
	"math/rand"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/notifier"
	"github.com/turt2live/matrix-media-repo/tasks/task_runner"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/ids"
)

type TaskName string
type RecurringTaskName string

const (
	TaskDatastoreMigrate TaskName = "storage_migration"
	TaskExportData       TaskName = "export_data"
)
const (
	RecurringTaskPurgeThumbnails  RecurringTaskName = "recurring_purge_thumbnails"
	RecurringTaskPurgePreviews    RecurringTaskName = "recurring_purge_previews"
	RecurringTaskPurgeRemoteMedia RecurringTaskName = "recurring_purge_remote_media"
)

type RecurringTaskFn func(ctx rcontext.RequestContext)

var localRand = rand.New(rand.NewSource(util.NowMillis()))
var recurDoneChs = make(map[RecurringTaskName]chan bool)
var recurLock = new(sync.RWMutex)

func scheduleTask(ctx rcontext.RequestContext, name TaskName, params interface{}) (*database.DbTask, error) {
	jsonParams := &database.AnonymousJson{}
	if err := jsonParams.ApplyFrom(params); err != nil {
		return nil, err
	}
	db := database.GetInstance().Tasks.Prepare(ctx)
	r, err := db.Insert(string(name), jsonParams, util.NowMillis())
	if err != nil {
		return nil, err
	}

	if ids.GetMachineId() == 0 {
		// we'll run the task on this machine too
		beginTask(r)
	} else {
		if err = notifier.TaskScheduled(ctx, r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func scheduleHourly(name RecurringTaskName, workFn RecurringTaskFn) {
	if ids.GetMachineId() != 0 {
		return // don't run tasks on non-zero machine IDs
	}

	ticker := time.NewTicker((1 * time.Hour) + (time.Duration(localRand.Intn(15)) * time.Minute))
	ch := make(chan bool)
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": name})
	recurLock.Lock()
	defer recurLock.Unlock()
	if val, ok := recurDoneChs[name]; ok {
		val <- true // close that channel
	}
	recurDoneChs[name] = ch
	go func() {
		defer close(ch)
		defer func() {
			recurLock.Lock()
			defer recurLock.Unlock()
			delete(recurDoneChs, name)
		}()

		for {
			select {
			case <-ch:
				ticker.Stop()
				return
			case <-ticker.C:
				workFn(ctx)
			}
		}
	}()
}

func stopRecurring() {
	recurLock.RLock()
	defer recurLock.RUnlock()
	for _, ch := range recurDoneChs {
		ch <- true
	}
}

func scheduleUnfinished() {
	if ids.GetMachineId() != 0 {
		return // don't schedule here
	}
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"startup": true})
	taskDb := database.GetInstance().Tasks.Prepare(ctx)
	tasks, err := taskDb.GetAll(false)
	if err != nil {
		sentry.CaptureException(err)
		ctx.Log.Fatal("Error getting unfinished tasks: ", err)
		return
	}
	for _, task := range tasks {
		beginTask(task)
	}
}

func RunDatastoreMigration(ctx rcontext.RequestContext, sourceDsId string, targetDsId string, beforeTs int64) (*database.DbTask, error) {
	return scheduleTask(ctx, TaskDatastoreMigrate, task_runner.DatastoreMigrateParams{
		SourceDsId: sourceDsId,
		TargetDsId: targetDsId,
		BeforeTs:   beforeTs,
	})
}

func RunUserExport(ctx rcontext.RequestContext, userId string, includeS3Urls bool) (*database.DbTask, string, error) {
	return runExport(ctx, task_runner.ExportDataParams{
		UserId:        userId,
		IncludeS3Urls: includeS3Urls,
		//ExportId:      "", // populated by runExport
	})
}

func RunServerExport(ctx rcontext.RequestContext, serverName string, includeS3Urls bool) (*database.DbTask, string, error) {
	return runExport(ctx, task_runner.ExportDataParams{
		ServerName:    serverName,
		IncludeS3Urls: includeS3Urls,
		//ExportId:      "", // populated by runExport
	})
}

func runExport(ctx rcontext.RequestContext, paramsTemplate task_runner.ExportDataParams) (*database.DbTask, string, error) {
	exportId, err := ids.NewUniqueId()
	if err != nil {
		return nil, "", err
	}
	paramsTemplate.ExportId = exportId
	task, err := scheduleTask(ctx, TaskExportData, paramsTemplate)
	return task, exportId, err
}
