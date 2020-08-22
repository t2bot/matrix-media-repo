package archival

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/templating"
	"github.com/turt2live/matrix-media-repo/util"
)

type V2ArchiveWriter interface {
	WritePart(part int, fileName string, archive io.Reader, size int64) error
}

type V2ArchiveExport struct {
	exportId      string
	entity        string
	indexModel    *templating.ExportIndexModel
	writer        V2ArchiveWriter
	mediaManifest map[string]*V2ManifestRecord
	partSize      int64
	ctx           rcontext.RequestContext

	// state variables
	currentPart     int
	currentTar      *tar.Writer
	currentTarBytes *bytes.Buffer
	currentSize     int64
	writingManifest bool
}

func NewV2Export(exportId string, entity string, partSize int64, writer V2ArchiveWriter, ctx rcontext.RequestContext) (*V2ArchiveExport, error) {
	ctx = ctx.LogWithFields(logrus.Fields{
		"v2_export-id":       exportId,
		"v2_export-entity":   entity,
		"v2_export-partSize": partSize,
	})
	archiver := &V2ArchiveExport{
		exportId: exportId,
		entity:   entity,
		writer:   writer,
		partSize: partSize,
		ctx: ctx,
		indexModel: &templating.ExportIndexModel{
			Entity:   entity,
			ExportID: exportId,
			Media:    make([]*templating.ExportIndexMediaModel, 0),
		},
		mediaManifest: make(map[string]*V2ManifestRecord),
		currentPart:   0,
	}
	ctx.Log.Info("Preparing first tar file...")
	err := archiver.newTar()
	return archiver, err
}

func (e *V2ArchiveExport) newTar() error {
	if e.currentPart > 0 {
		e.ctx.Log.Info("Persisting complete tar file...")
		if err := e.persistTar(); err != nil {
			return err
		}
	}

	e.ctx.Log.Info("Starting new tar file...")
	e.currentTarBytes = &bytes.Buffer{}
	e.currentTar = tar.NewWriter(e.currentTarBytes)
	e.currentPart = e.currentPart + 1
	e.currentSize = 0

	return nil
}

func (e *V2ArchiveExport) persistTar() error {
	_ = e.currentTar.Close()

	e.ctx.Log.Info("Compressing tar file...")
	gzipBytes := &bytes.Buffer{}
	archiver := gzip.NewWriter(gzipBytes)
	archiver.Name = fmt.Sprintf("export-part-%d.tar", e.currentPart)
	if e.writingManifest {
		archiver.Name = "export-manifest.tar"
	}

	if _, err := io.Copy(archiver, util.ClonedBufReader(*e.currentTarBytes)); err != nil {
		return err
	}
	_ = archiver.Close()

	e.ctx.Log.Info("Writing compressed tar")
	name := fmt.Sprintf("export-part-%d.tgz", e.currentPart)
	if e.writingManifest {
		name = "export-manifest.tgz"
	}
	return e.writer.WritePart(e.currentPart, name, gzipBytes, int64(len(gzipBytes.Bytes())))
}

func (e *V2ArchiveExport) putFile(buf *bytes.Buffer, name string, creationTime time.Time) (int64, error) {
	length := int64(len(buf.Bytes()))
	header := &tar.Header{
		Name:    name,
		Size:    length,
		Mode:    int64(0644),
		ModTime: creationTime,
	}
	if err := e.currentTar.WriteHeader(header); err != nil {
		return 0, err
	}

	i, err := io.Copy(e.currentTar, buf)
	if err != nil {
		return 0, err
	}
	e.currentSize += i

	return length, nil
}

func (e *V2ArchiveExport) AppendMedia(origin string, mediaId string, originalName string, contentType string, creationTime time.Time, file io.Reader, sha256 string, s3Url string, userId string) error {
	// buffer the entire file into memory
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, file); err != nil {
		return err
	}

	mime := mimetype.Detect(buf.Bytes())
	internalName := fmt.Sprintf("%s__%s%s", origin, mediaId, mime.Extension())

	length, err := e.putFile(buf, internalName, creationTime)
	if err != nil {
		return err
	}

	mxc := fmt.Sprintf("mxc://%s/%s", origin, mediaId)
	e.mediaManifest[mxc] = &V2ManifestRecord{
		ArchivedName: internalName,
		FileName:     originalName,
		SizeBytes:    length,
		ContentType:  contentType,
		S3Url:        s3Url,
		Sha256:       sha256,
		Origin:       origin,
		MediaId:      mediaId,
		CreatedTs:    creationTime.UnixNano() / 1000000,
		Uploader:     userId,
	}
	e.indexModel.Media = append(e.indexModel.Media, &templating.ExportIndexMediaModel{
		ExportID:        e.exportId,
		ArchivedName:    internalName,
		FileName:        originalName,
		SizeBytes:       length,
		SizeBytesHuman:  humanize.Bytes(uint64(length)),
		Origin:          origin,
		MediaID:         mediaId,
		Sha256Hash:      sha256,
		ContentType:     contentType,
		UploadTs:        creationTime.UnixNano() / 1000000,
		UploadDateHuman: creationTime.Format(time.UnixDate),
		Uploader:        userId,
	})

	if e.currentSize >= e.partSize {
		e.ctx.Log.Info("Rotating tar...")
		return e.newTar()
	}

	return nil
}

func (e *V2ArchiveExport) Finish() error {
	if err := e.newTar(); err != nil {
		return err
	}

	e.ctx.Log.Info("Writing manifest...")
	e.writingManifest = true
	defer (func() { e.writingManifest = false })()
	manifest := &V2Manifest{
		Version:   2,
		EntityId:  e.entity,
		CreatedTs: util.NowMillis(),
		Media:     e.mediaManifest,
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		e.writingManifest = false
		return err
	}
	if _, err := e.putFile(bytes.NewBuffer(b), "manifest.json", time.Now()); err != nil {
		return err
	}

	e.ctx.Log.Info("Writing index...")
	t, err := templating.GetTemplate("export_index")
	if err != nil {
		return err
	}
	html := bytes.Buffer{}
	if err := t.Execute(&html, e.indexModel); err != nil {
		return err
	}
	if _, err := e.putFile(bytes.NewBuffer(html.Bytes()), "index.html", time.Now()); err != nil {
		return err
	}

	e.ctx.Log.Info("Writing manifest tar...")
	return e.persistTar()
}
