package _common

import (
	"io"
	"os"
	"path"

	"github.com/t2bot/matrix-media-repo/archival/v2archive"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func ProcessArchiveDirectory(ctx rcontext.RequestContext, entityId string, directory string, processFn func(record *v2archive.ManifestRecord, f io.ReadCloser) error) error {
	ctx.Log.Info("Discovering files...")
	fileInfos, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	files := make([]string, 0)
	for _, f := range fileInfos {
		if f.IsDir() {
			continue
		}
		files = append(files, path.Join(directory, f.Name()))
	}

	archiver := v2archive.NewReader(ctx)

	// Find the manifest
	for _, fname := range files {
		ctx.Log.Debugf("Scanning %s for manifest", fname)
		f, err := os.Open(fname)
		if err != nil {
			return err
		}
		if ok, err := archiver.TryGetManifestFrom(f); err != nil {
			return err
		} else if ok {
			break
		}
	}
	if len(archiver.GetNotUploadedMxcUris()) <= 0 {
		ctx.Log.Warn("Found zero or fewer MXC URIs to process. This usually means there was no manifest found.")
		return nil
	}
	ctx.Log.Info("Importing media for %s", archiver.GetEntityId())

	// Re-process all the files properly
	opts := v2archive.ProcessOpts{
		LockedEntityId:    entityId,
		CheckUploadedOnly: false,
		ProcessFunction:   processFn,
	}
	for _, fname := range files {
		ctx.Log.Debugf("Processing %s for media", fname)
		f, err := os.Open(fname)
		if err != nil {
			return err
		}
		if err = archiver.ProcessFile(f, opts); err != nil {
			return err
		}
	}
	if err = archiver.ProcessS3Files(opts); err != nil {
		return err
	}

	missing := archiver.GetNotUploadedMxcUris()
	if len(missing) > 0 {
		for _, mxc := range missing {
			ctx.Log.Warnf("%s has not been uploaded yet - was it included in the package?", mxc)
		}
		ctx.Log.Warnf("%d MXC URIs have not been imported.", len(missing))
	}

	return nil
}
