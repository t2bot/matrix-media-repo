package v2archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/turt2live/matrix-media-repo/util"
)

type ProcessOpts struct {
	LockedEntityId    string
	CheckUploadedOnly bool
}

type ArchiveReader struct {
	ctx rcontext.RequestContext

	manifest        *Manifest
	uploaded        map[string]bool
	fileNamesToMxcs map[string][]string
}

func NewReader(ctx rcontext.RequestContext) *ArchiveReader {
	return &ArchiveReader{
		ctx:             ctx,
		manifest:        nil,
		uploaded:        make(map[string]bool),
		fileNamesToMxcs: make(map[string][]string),
	}
}

type archiveWorkFn = func(header *tar.Header, f io.Reader) error

func readArchive(file io.ReadCloser, workFn archiveWorkFn) error {
	archiver, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func(archiver *gzip.Reader) {
		_ = archiver.Close()
	}(archiver)

	tarFile := tar.NewReader(archiver)
	for {
		header, err := tarFile.Next()
		if err == io.EOF {
			break // we're done
		}
		if err != nil {
			return err
		}

		if header == nil {
			continue // skip weird file
		}
		if header.Typeflag != tar.TypeReg {
			continue // skip directories and other stuff
		}

		err = workFn(header, tarFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ArchiveReader) GetEntityId() string {
	if r.manifest != nil {
		return r.manifest.EntityId
	}
	return ""
}

func (r *ArchiveReader) GetNotUploadedMxcUris() []string {
	uris := make([]string, 0)
	for k, v := range r.uploaded {
		if !v {
			uris = append(uris, k)
		}
	}
	return uris
}

func (r *ArchiveReader) TryGetManifestFrom(file io.ReadCloser) (bool, error) {
	defer file.Close()
	if r.manifest != nil {
		return false, errors.New("manifest already discovered")
	}

	err := readArchive(file, func(header *tar.Header, f io.Reader) error {
		if header.Name == "manifest.json" {
			manifest := &Manifest{}
			decoder := json.NewDecoder(f)
			err := decoder.Decode(&manifest)
			if err != nil {
				return err
			}
			if manifest.Version == ManifestVersionV1 {
				manifest.EntityId = manifest.UserId
				manifest.Version = ManifestVersionV2
				r.ctx.Log.Debug("Upgraded manifest to v2")
			}
			if manifest.Version != ManifestVersionV2 {
				// We only support the one version for now.
				return errors.New("unsupported manifest version")
			}
			if manifest.EntityId == "" {
				return errors.New("invalid manifest: no entity")
			}
			if manifest.Media == nil {
				return errors.New("invalid manifest: no media")
			}
			r.ctx.Log.Infof("Using manifest for %s (v%d) created %d", manifest.EntityId, manifest.Version, manifest.CreatedTs)
			r.manifest = manifest

			for k, v := range r.manifest.Media {
				r.uploaded[k] = false
				if _, ok := r.fileNamesToMxcs[v.ArchivedName]; !ok {
					r.fileNamesToMxcs[v.ArchivedName] = make([]string, 0)
				}
				r.fileNamesToMxcs[v.ArchivedName] = append(r.fileNamesToMxcs[v.ArchivedName], k)
			}

			return nil
		}
		return nil
	})
	return r.manifest != nil, err
}

func (r *ArchiveReader) ProcessS3Files(opts ProcessOpts) error {
	if r.manifest == nil {
		return errors.New("missing manifest")
	}

	missing := r.GetNotUploadedMxcUris()
	for _, mxc := range missing {
		metadata := r.manifest.Media[mxc]
		if metadata.S3Url != "" {
			dsConf, location, err := datastores.ParseS3Url(metadata.S3Url)
			if err != nil {
				r.ctx.Log.Warn("Error while parsing S3 URL for ", mxc, err)
				continue
			}
			f, err := datastores.Download(r.ctx, dsConf, location)
			if err != nil {
				r.ctx.Log.Warn("Error while downloading from S3 URL for ", mxc, err)
				continue
			}
			err = r.importFileFromStream(metadata.FileName, f, opts)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ArchiveReader) ProcessFile(file io.ReadCloser, opts ProcessOpts) error {
	defer file.Close()
	if r.manifest == nil {
		return errors.New("missing manifest")
	}

	err := readArchive(file, func(header *tar.Header, f io.Reader) error {
		r.ctx.Log.Debugf("Processing file in tar: %s", header.Name)
		return r.importFileFromStream(header.Name, io.NopCloser(f), opts)
	})
	return err
}

func (r *ArchiveReader) importFileFromStream(fileName string, f io.ReadCloser, opts ProcessOpts) error {
	mxcs, ok := r.fileNamesToMxcs[fileName]
	if !ok {
		r.ctx.Log.Debugf("File %s does not map to an MXC URI", fileName)
		return nil // ignore file
	}

	db := database.GetInstance().Media.Prepare(r.ctx)
	for _, mxc := range mxcs {
		if r.uploaded[mxc] {
			continue // ignore duplicate file
		}

		metadata := r.manifest.Media[mxc]
		genMxc := util.MxcUri(metadata.Origin, metadata.MediaId)
		if genMxc != mxc {
			r.ctx.Log.Warnf("File name maps to %s but expected %s from metadata - skipping file", genMxc, mxc)
			continue
		}
		if metadata.Uploader != "" {
			_, s, err := util.SplitUserId(metadata.Uploader)
			if err != nil {
				r.ctx.Log.Warnf("Invalid user ID in metadata: %s (media %s)", metadata.Uploader, mxc)
				metadata.Uploader = ""
			} else {
				if s != metadata.Origin {
					r.ctx.Log.Warnf("File has uploader on %s but MXC URI is for %s - skipping file", s, metadata.Origin)
					continue
				}
			}
		}

		if opts.LockedEntityId != "" {
			if opts.LockedEntityId[0] == '@' && metadata.Uploader != opts.LockedEntityId {
				r.ctx.Log.Warnf("Found media uploaded by %s but locked to %s - skipping file", metadata.Uploader, opts.LockedEntityId)
				continue
			}
			if opts.LockedEntityId[0] != '@' && metadata.Origin != opts.LockedEntityId {
				r.ctx.Log.Warnf("Found media uploaded by server %s but locked to %s - skipping file", metadata.Origin, opts.LockedEntityId)
				continue
			}
		}

		record, err := db.GetById(metadata.Origin, metadata.MediaId)
		if err != nil {
			return err
		}
		if record != nil {
			r.uploaded[mxc] = true
			continue
		}

		if opts.CheckUploadedOnly {
			continue
		}

		serverName := r.manifest.EntityId
		userId := metadata.Uploader
		if userId[0] != '@' {
			userId = ""
		} else {
			_, s, err := util.SplitUserId(userId)
			if err != nil {
				r.ctx.Log.Warnf("Invalid user ID: %s (media %s)", userId, mxc)
				serverName = ""
			} else {
				serverName = s
			}
		}
		kind := datastores.LocalMediaKind
		if !util.IsServerOurs(serverName) {
			kind = datastores.RemoteMediaKind
		}

		r.ctx.Log.Debugf("Importing file %s as kind %s", mxc, kind)
		if _, err = pipeline_upload.Execute(r.ctx, metadata.Origin, metadata.MediaId, f, metadata.ContentType, metadata.FileName, metadata.Uploader, kind); err != nil {
			return err
		}
		r.uploaded[mxc] = true
	}

	return nil
}
