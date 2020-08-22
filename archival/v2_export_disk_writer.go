package archival

import (
	"io"
	"os"
	"path"
)

type V2ArchiveDiskWriter struct {
	directory string
}

func NewV2ArchiveDiskWriter(directory string) *V2ArchiveDiskWriter {
	return &V2ArchiveDiskWriter{directory: directory}
}

func (w V2ArchiveDiskWriter) WritePart(part int, fileName string, archive io.Reader, size int64) error {
	f, err := os.Create(path.Join(w.directory, fileName))
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, archive); err != nil {
		return err
	}
	return f.Close()
}
