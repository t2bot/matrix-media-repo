package tasks

import (
	"math/rand"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/notifier"
	"github.com/t2bot/matrix-media-repo/tasks/task_runner"
	"github.com/t2bot/matrix-media-repo/util/ids"
)

type TaskName string
type RecurringTaskName string

const (
	TaskDatastoreMigrate TaskName = "storage_migration"
	TaskExportData       TaskName = "export_data"
	TaskImportData       TaskName = "import_data"
)
const (
	RecurringTaskPurgeThumbnails   RecurringTaskName = "recurring_purge_thumbnails"
	RecurringTaskPurgePreviews     RecurringTaskName = "recurring_purge_previews"
	RecurringTaskPurgeRemoteMedia  RecurringTaskName = "recurring_purge_remote_media"
	RecurringTaskPurgeHeldMediaIds RecurringTaskName = "recurring_purge_held_media_ids"
)

const ExecutingMachineId = int64(0)

type RecurringTaskFn func(ctx rcontext.RequestContext)

var localRand = rand.New(rand.NewSource(time.Now().UnixNano()))
var recurDoneChs = make(map[RecurringTaskName]chan bool)
var recurLock = new(sync.RWMutex)

func scheduleTask(ctx rcontext.RequestContext, name TaskName, params interface{}) (*database.DbTask, error) {
	jsonParams := &database.AnonymousJson{}
	if err := jsonParams.ApplyFrom(params); err != nil {
		return nil, err
	}
	db := database.GetInstance().Tasks.Prepare(ctx)
	r, err := db.Insert(string(name), jsonParams, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}

	if ids.GetMachineId() == ExecutingMachineId {
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
	if ids.GetMachineId() != ExecutingMachineId {
		return // don't run tasks on this machine
	}

	ticker := time.NewTicker((1 * time.Hour) + (time.Duration(localRand.Intn(15)) * time.Minute))
	ch := make(chan bool)
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": name})
	recurLock.Lock()
	defer recurLock.Unlock()
	if val, ok := recurDoneChs[name]; ok {
		// Check if closed, and close if needed
		select {
		case <-val:
			break // already closed
		default:
			val <- true // close that channel
		}
	}
	recurDoneChs[name] = ch
	go func() {
		defer func() {
			recurLock.Lock()
			defer recurLock.Unlock()
			close(ch)
			if recurDoneChs[name] == ch {
				delete(recurDoneChs, name)
			}
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
	if ids.GetMachineId() != ExecutingMachineId {
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
		go beginTask(task)
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

func RunImport(ctx rcontext.RequestContext) (*database.DbTask, string, error) {
	importId, err := ids.NewUniqueId()
	if err != nil {
		return nil, "", err
	}
	task, err := scheduleTask(ctx, TaskImportData, task_runner.ImportDataParams{
		ImportId: importId,
	})
	return task, importId, err
}
