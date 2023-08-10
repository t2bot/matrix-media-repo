package test_internals

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sabhiram/go-gitignore"
)

func createDockerContext() (*os.File, error) {
	ignoreFile, err := ignore.CompileIgnoreFile(".dockerignore")
	if err != nil {
		return nil, err
	}

	tmpF, err := os.CreateTemp(os.TempDir(), "mmr-docker-context")
	tarContext := tar.NewWriter(tmpF)

	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		if match := ignoreFile.MatchesPath(path); match {
			return nil
		}
		//fmt.Println("[Image Build] Including file: ", path)
		err = tarContext.WriteHeader(&tar.Header{
			Name:    strings.ReplaceAll(path, "\\", "/"),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
		if err != nil {
			return err
		}
		f2, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f2.Close()
		_, err = io.Copy(tarContext, f2)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = tarContext.Close()
	if err != nil {
		return nil, err
	}

	err = tmpF.Close()
	if err != nil {
		return nil, err
	}

	return os.Open(tmpF.Name())
}
