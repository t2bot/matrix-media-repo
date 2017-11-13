package util

import "os"

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
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