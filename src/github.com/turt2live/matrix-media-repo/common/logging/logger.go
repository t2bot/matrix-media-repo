package logging

import (
	"os"
	"path"
	"time"

	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

type utcFormatter struct {
	logrus.Formatter
}

func (f utcFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	entry.Time = entry.Time.UTC()
	return f.Formatter.Format(entry)
}

func Setup(dir string) error {
	logrus.SetFormatter(&utcFormatter{
		&logrus.TextFormatter{
			TimestampFormat:  "2006-01-02 15:04:05.000 Z07:00",
			FullTimestamp:    true,
			ForceColors:      true,
			DisableColors:    false,
			DisableTimestamp: false,
			QuoteEmptyFields: true,
		},
	})
	logrus.SetOutput(os.Stdout)

	if dir == "" {
		return nil
	}
	_ = os.MkdirAll(dir, os.ModePerm)

	logFile := path.Join(dir, "media_repo.log")
	writer, err := rotatelogs.New(
		logFile+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(logFile),
		rotatelogs.WithMaxAge((24*time.Hour)*14),  // keep for 14 days
		rotatelogs.WithRotationTime(24*time.Hour), // rotate every 24 hours
	)
	if err != nil {
		return err
	}

	logrus.AddHook(lfshook.NewHook(lfshook.WriterMap{
		logrus.DebugLevel: writer,
		logrus.InfoLevel:  writer,
		logrus.WarnLevel:  writer,
		logrus.ErrorLevel: writer,
		logrus.FatalLevel: writer,
		logrus.PanicLevel: writer,
	}))

	return nil
}
