package logging

import (
	"os"
	"path"
	"time"

	"github.com/DavidHuie/gomigrate"
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

func Setup(dir string, colors bool, json bool, level string) error {
	if level == "" {
		level = "info"
	}
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)

	var lineFormatter logrus.Formatter
	if json {
		lineFormatter = &logrus.JSONFormatter{
			TimestampFormat:  "2006-01-02 15:04:05.000 Z07:00",
			DisableTimestamp: false,
		}
	} else {
		lineFormatter = &logrus.TextFormatter{
			TimestampFormat:  "2006-01-02 15:04:05.000 Z07:00",
			FullTimestamp:    true,
			ForceColors:      colors,
			DisableColors:    !colors,
			DisableTimestamp: false,
			QuoteEmptyFields: true,
		}
	}
	formatter := &utcFormatter{lineFormatter}
	logrus.SetFormatter(formatter)
	logrus.SetOutput(os.Stdout)

	if dir == "" || dir == "-" {
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
	}, formatter))

	return nil
}

type SendToDebugLogger struct {
	gomigrate.Logger
	//ants.Logger
}

func (*SendToDebugLogger) Print(v ...interface{}) {
	logrus.Debug(v...)
}

func (*SendToDebugLogger) Printf(format string, v ...interface{}) {
	logrus.Debugf(format, v...)
}

func (*SendToDebugLogger) Println(v ...interface{}) {
	logrus.Debugln(v...)
}

func (*SendToDebugLogger) Fatalf(format string, v ...interface{}) {
	logrus.Fatalf(format, v...)
}
