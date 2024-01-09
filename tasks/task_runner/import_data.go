package task_runner

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/archival/v2archive"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

var importEngines = new(sync.Map)

type ImportDataParams struct {
	ImportId string `json:"export_id"`
}

type importEngine struct {
	importId string
	task     *database.DbTask
	ctx      rcontext.RequestContext
	archiver *v2archive.ArchiveReader
	files    []*os.File
	mutex    *sync.Mutex
	workCh   chan<- *os.File
}

func createEngine(ctx rcontext.RequestContext, task *database.DbTask) error {
	params := ImportDataParams{}
	if err := task.Params.ApplyTo(&params); err != nil {
		return err
	}

	if params.ImportId == "" {
		return errors.New("no import ID specified")
	}

	if _, ok := importEngines.Load(params.ImportId); ok {
		return errors.New("import already in progress")
	}

	ch := make(chan *os.File)
	engine := &importEngine{
		importId: params.ImportId,
		task:     task,
		ctx:      ctx,
		archiver: v2archive.NewReader(ctx),
		files:    make([]*os.File, 0),
		mutex:    new(sync.Mutex),
		workCh:   ch,
	}
	importEngines.Store(params.ImportId, engine)
	go engine.workFn(ch)

	return nil
}

func (e *importEngine) appendFile(data io.ReadCloser) error {
	// Dump the import to a file first
	f, err := os.CreateTemp(os.TempDir(), "mmr-import")
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, data); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	// Reopen the file for reading
	f, err = os.Open(f.Name())
	if err != nil {
		return err
	}

	// Store the file
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.files = append(e.files, f)
	go func() {
		e.workCh <- f
	}()
	return nil
}

func (e *importEngine) finish(chClose bool, err error) error {
	if err != nil {
		markError(e.ctx, e.task, err)
	}
	markDone(e.ctx, e.task)
	importEngines.Delete(e.importId)
	if chClose {
		e.workCh <- nil // clean up goroutine
	}
	return nil
}

func (e *importEngine) workFn(ch chan *os.File) {
	defer close(ch)
	defer func() {
		for _, f := range e.files {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	for f := range ch {
		if f == nil {
			return
		}

		if !e.archiver.HasManifest() {
			e.ctx.Log.Debug("Trying newly received file for manifest")
			ok, err := e.archiver.TryGetManifestFrom(readers.NewRewindReader(f))
			if err != nil {
				e.ctx.Log.Error("Error during manifest search: ", err)
				sentry.CaptureException(err)
				_ = e.finish(false, errors.Join(errors.New("error in search"), err))
				return
			}
			if ok {
				e.ctx.Log.Debug("Manifest found! Processing known files")
				for _, f2 := range e.files { // e.files should include our current file
					if err = e.archiver.ProcessFile(f2, v2archive.ProcessOpts{}); err != nil {
						e.ctx.Log.Error("Error during file processing (branch-1): ", err)
						sentry.CaptureException(err)
						_ = e.finish(false, errors.Join(errors.New("error in processing"), err))
						return
					}
				}
			}
		} else {
			e.ctx.Log.Debug("Manifest already known! Processing new file")
			if err := e.archiver.ProcessFile(f, v2archive.ProcessOpts{}); err != nil {
				e.ctx.Log.Error("Error during file processing (branch-2): ", err)
				sentry.CaptureException(err)
				_ = e.finish(false, errors.Join(errors.New("error in processing2"), err))
				return
			}
		}

		if e.archiver.HasManifest() && len(e.archiver.GetNotUploadedMxcUris()) == 0 {
			e.ctx.Log.Debug("No more files waiting for import - closing engine")
			_ = e.finish(false, nil)
			return
		}
	}
}

func ImportData(ctx rcontext.RequestContext, task *database.DbTask) {
	if err := createEngine(ctx, task); err != nil {
		markDone(ctx, task) // forcefully mark the task as done since there was an error
		ctx.Log.Error("Error creating import engine: ", err)
		sentry.CaptureException(err)
	}
}

func AppendImportFile(ctx rcontext.RequestContext, importId string, data io.ReadCloser) error {
	try := func() error {
		if val, ok := importEngines.Load(importId); !ok {
			return common.ErrMediaNotFound
		} else if engine, ok := val.(*importEngine); !ok {
			return errors.New("logic error: non-engine stored")
		} else if engine != nil {
			return engine.appendFile(data)
		}
		return errors.New("logic error: missed engine lookup")
	}
	// We give a few tries before giving up, as there's a good chance the
	// caller *just* started the import job and is already trying to append.
	for i := 0; i < 5; i++ {
		err := try()
		if errors.Is(err, common.ErrMediaNotFound) {
			time.Sleep(100 * time.Millisecond)
		} else {
			return err
		}
	}
	return common.ErrMediaNotFound
}

func FinishImport(ctx rcontext.RequestContext, importId string) error {
	if val, ok := importEngines.Load(importId); !ok {
		return common.ErrMediaNotFound
	} else if engine, ok := val.(*importEngine); !ok {
		return errors.New("logic error: non-engine stored")
	} else if engine != nil {
		return engine.finish(true, nil)
	}
	return errors.New("logic error: missed engine lookup")
}
