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

func GetLastSegmentsOfPath(strPath string, segments int) string {
	combined := ""
	for i := 1; i <= segments; i++ {
		d, p := path.Split(strPath)
		strPath = path.Clean(d)
		combined = path.Join(p, combined)
	}
	return combined
}

func GetFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return GetSha256HashOfStream(f)
}
