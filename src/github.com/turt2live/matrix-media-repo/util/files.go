package util

import (
	"os"
	"path"
)

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func FileSize(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		os.Remove(path)
		return 0, err
	}

	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}

	return fi.Size(), nil
}

func GetLastSegmentsOfPath(strPath string, segments int) (string) {
	combined := ""
	for i := 1; i <= segments; i++ {
		d, p := path.Split(strPath)
		strPath = path.Clean(d)
		combined = path.Join(p, combined)
	}
	return combined
}
