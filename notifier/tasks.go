package notifier

import (
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/redislib"
)

type TaskId int

const tasksNotifyRedisChannel = "mmr:bg_tasks"

func SubscribeToTasks() <-chan TaskId {
	ch := redislib.Subscribe(tasksNotifyRedisChannel)
	if ch == nil {
		return nil
	}

	retCh := make(chan TaskId)
	go func() {
		for val := range ch {
			if i, err := strconv.Atoi(val); err != nil {
				sentry.CaptureException(err)
				logrus.Error("Internal error handling tasks subscribe: ", err)
			} else {
				retCh <- TaskId(i)
			}
		}
	}()
	return retCh
}

func TaskScheduled(ctx rcontext.RequestContext, task *database.DbTask) error {
	return redislib.Publish(ctx, tasksNotifyRedisChannel, strconv.Itoa(task.TaskId))
}
