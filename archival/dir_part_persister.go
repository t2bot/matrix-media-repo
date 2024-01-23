package archival

import (
	"io"
	"os"
	"path"

	"github.com/t2bot/matrix-media-repo/archival/v2archive"
)

func PersistPartsToDirectory(exportPath string) v2archive.PartPersister {
	_ = os.MkdirAll(exportPath, 0755)
	return func(part int, fileName string, data io.ReadCloser) error {
		defer data.Close()
		f, errf := os.Create(path.Join(exportPath, fileName))
		if errf != nil {
			return errf
		}
		_, errf = io.Copy(f, data)
		if errf != nil {
			return errf
		}
		return nil
	}
}
