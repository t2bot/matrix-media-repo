package pool

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/logging"
)

type Queue struct {
	pool *ants.Pool
}

func NewQueue(workers int, name string) (*Queue, error) {
	p, err := ants.NewPool(workers, ants.WithOptions(ants.Options{
		ExpiryDuration:   1 * time.Minute, // worker lifespan when unused
		PreAlloc:         false,
		MaxBlockingTasks: 0, // no limit on tasks we can submit
		Nonblocking:      false,
		PanicHandler: func(err interface{}) {
			logrus.Errorf("Panic from internal queue %s", name)
			logrus.Error(err)
			//goland:noinspection GoTypeAssertionOnErrors
			if e, ok := err.(error); ok {
				sentry.CaptureException(e)
			}
		},
		Logger:       &logging.SendToDebugLogger{},
		DisablePurge: false,
	}))
	if err != nil {
		return nil, err
	}
	return &Queue{pool: p}, nil
}

func (p *Queue) Schedule(task func()) error {
	return p.pool.Submit(task)
}
