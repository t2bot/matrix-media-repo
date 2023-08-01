package tasks

import (
	"github.com/turt2live/matrix-media-repo/tasks/task_runner"
)

func StartAll() {
	executeEnable()

	scheduleHourly(RecurringTaskPurgeRemoteMedia, task_runner.PurgeRemoteMedia)
	scheduleHourly(RecurringTaskPurgeThumbnails, task_runner.PurgeThumbnails)
	scheduleHourly(RecurringTaskPurgePreviews, task_runner.PurgePreviews)

	scheduleUnfinished()
}

func StopAll() {
	stopRecurring()
}
