package u

import (
	"errors"
	"io"
	"os"

	"github.com/dhowden/tag"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

func GetID3Tags(b io.Reader) (tag.Metadata, io.ReadSeekCloser, error) {
	var f *os.File
	var err error

	tryCleanup := func() {
		if f != nil {
			if err = os.Remove(f.Name()); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("Error deleting temp file '%s': %s", f.Name(), err.Error())
				sentry.CaptureException(errors.New("id3: error deleting temp file: " + err.Error()))
			}
		}
	}

	f, err = os.CreateTemp(os.TempDir(), "mmr-id3")
	if err != nil {
		tryCleanup()
		return nil, nil, err
	}
	if _, err = io.Copy(f, b); err != nil {
		tryCleanup()
		return nil, nil, err
	}
	if err = f.Close(); err != nil {
		tryCleanup()
		return nil, nil, err
	}
	if f, err = os.OpenFile(f.Name(), os.O_WRONLY, 0644); err != nil {
		tryCleanup()
		return nil, nil, err
	}

	meta, _ := tag.ReadFrom(f) // we don't care about errors in this process
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		tryCleanup()
		return nil, nil, err
	}

	return meta, readers.NewTempFileCloser("", f.Name(), f), nil
}
