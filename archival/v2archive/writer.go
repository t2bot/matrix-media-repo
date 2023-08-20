package v2archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/templating"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

type MediaInfo struct {
	Origin      string
	MediaId     string
	FileName    string
	ContentType string
	CreationTs  int64
	S3Url       string
	UserId      string
}

type PartPersister func(part int, fileName string, data io.ReadCloser) error

type ArchiveWriter struct {
	ctx rcontext.RequestContext

	exportId      string
	entity        string
	index         *templating.ExportIndexModel
	mediaManifest map[string]*ManifestRecord
	partSize      int64
	writeFn       PartPersister

	// state machine variables
	currentPart     int
	currentTar      *tar.Writer
	currentTempFile *os.File
	currentSize     int64
	writingManifest bool
}

func NewWriter(ctx rcontext.RequestContext, exportId string, entity string, partSize int64, writeFn PartPersister) (*ArchiveWriter, error) {
	ctx = ctx.LogWithFields(logrus.Fields{
		"v2archive-id":     exportId,
		"v2archive-entity": entity,
	})
	archiver := &ArchiveWriter{
		ctx:      ctx,
		exportId: exportId,
		entity:   entity,
		index: &templating.ExportIndexModel{
			ExportID: exportId,
			Entity:   entity,
			Media:    make([]*templating.ExportIndexMediaModel, 0),
		},
		mediaManifest: make(map[string]*ManifestRecord),
		partSize:      partSize,
		writeFn:       writeFn,
		currentPart:   0,
	}
	err := archiver.beginTar()
	return archiver, err
}

func (w *ArchiveWriter) rotateTar() error {
	if w.currentPart > 0 {
		if err := w.writeTar(); err != nil {
			return err
		}
	}

	return w.beginTar()
}

func (w *ArchiveWriter) beginTar() error {
	w.currentSize = 0
	w.currentPart = w.currentPart + 1

	file, err := os.CreateTemp(os.TempDir(), "mmr-archive")
	if err != nil {
		return err
	}

	w.currentTempFile = file
	w.currentTar = tar.NewWriter(file)
	return nil
}

func (w *ArchiveWriter) writeTar() error {
	_ = w.currentTar.Close()

	tempFilePath := w.currentTempFile.Name()
	if err := w.currentTempFile.Close(); err != nil {
		return err
	}
	f, err := os.Open(tempFilePath)
	if err != nil {
		return err
	}

	pr, pw := io.Pipe()
	archiver := gzip.NewWriter(pw)
	fname := fmt.Sprintf("export-part-%d", w.currentPart)
	if w.writingManifest {
		fname = "export-manifest"
	}
	archiver.Name = fname + ".tar"

	w.ctx.Log.Debug("Writing tar file to gzip container: ", archiver.Name)

	go func() {
		_, err := io.Copy(archiver, f)
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			err = archiver.Close()
			if err != nil {
				_ = pw.CloseWithError(err)
			} else {
				_ = pw.Close()
			}
		}
	}()

	closerStack := readers.NewCancelCloser(pr, func() {
		_ = readers.NewTempFileCloser("", f.Name(), f).Close()
	})
	return w.writeFn(w.currentPart, fname+".tgz", closerStack)
}

// AppendMedia / returns (sha256hash, error)
func (w *ArchiveWriter) AppendMedia(file io.ReadCloser, info MediaInfo) (string, error) {
	defer file.Close()
	br := readers.NewBufferReadsReader(file)
	mime, err := mimetype.DetectReader(br)
	if err != nil {
		return "", err
	}
	internalName := fmt.Sprintf("%s__%s%s", info.Origin, info.MediaId, mime.Extension())

	createTime := util.FromMillis(info.CreationTs)

	size, sha256hash, err := w.putFile(br.GetRewoundReader(), internalName, createTime)
	if err != nil {
		return "", err
	}
	w.mediaManifest[util.MxcUri(info.Origin, info.MediaId)] = &ManifestRecord{
		FileName:     info.FileName,
		ArchivedName: internalName,
		SizeBytes:    size,
		ContentType:  info.ContentType,
		S3Url:        info.S3Url,
		Sha256:       sha256hash,
		Origin:       info.Origin,
		MediaId:      info.MediaId,
		CreatedTs:    info.CreationTs,
		Uploader:     info.UserId,
	}
	w.index.Media = append(w.index.Media, &templating.ExportIndexMediaModel{
		ExportID:        w.exportId,
		ArchivedName:    internalName,
		FileName:        html.EscapeString(info.FileName),
		Origin:          info.Origin,
		MediaID:         info.MediaId,
		SizeBytes:       size,
		SizeBytesHuman:  humanize.Bytes(uint64(size)),
		UploadTs:        info.CreationTs,
		UploadDateHuman: createTime.UTC().Format(time.UnixDate),
		Sha256Hash:      sha256hash,
		ContentType:     info.ContentType,
		Uploader:        info.UserId,
	})

	if w.currentSize >= w.partSize {
		return sha256hash, w.rotateTar()
	}

	return sha256hash, nil
}

func (w *ArchiveWriter) putFile(r io.Reader, name string, creationTime time.Time) (int64, string, error) {
	f, err := os.CreateTemp(os.TempDir(), "mmr-archive-put")
	if err != nil {
		return 0, "", err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	i1, err := io.Copy(f, r)
	if err != nil {
		return 0, "", err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return 0, "", err
	}

	hasher := sha256.New()
	header := &tar.Header{
		Name:    name,
		Mode:    int64(0644),
		ModTime: creationTime,
		Size:    i1,
	}
	if err := w.currentTar.WriteHeader(header); err != nil {
		return 0, "", err
	}

	mw := io.MultiWriter(hasher, w.currentTar)
	i2, err := io.Copy(mw, f)
	if err != nil {
		return 0, "", err
	}
	w.currentSize = w.currentSize + i2

	if i1 != i2 {
		w.ctx.Log.Warnf("Size mismatch! Expected %d bytes but wrote %d instead", i1, i2)
	}

	return i2, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (w *ArchiveWriter) Finish() error {
	if err := w.rotateTar(); err != nil {
		return err
	}

	w.writingManifest = true
	defer func() { w.writingManifest = false }()
	manifest := &Manifest{
		Version:   ManifestVersion,
		EntityId:  w.entity,
		CreatedTs: util.NowMillis(),
		Media:     w.mediaManifest,
	}
	pr, pw := io.Pipe()
	jenc := json.NewEncoder(pw)
	go func() {
		if err := jenc.Encode(manifest); err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()
	if _, _, err := w.putFile(pr, "manifest.json", time.Now()); err != nil {
		return err
	}

	t, err := templating.GetTemplate("export_index")
	if err != nil {
		return err
	}

	pr, pw = io.Pipe()
	go func() {
		if err := t.Execute(pw, w.index); err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()
	if _, _, err := w.putFile(pr, "index.html", time.Now()); err != nil {
		return err
	}

	return w.writeTar()
}
