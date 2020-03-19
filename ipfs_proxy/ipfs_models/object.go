package ipfs_models

import (
	"io"
)

type IPFSObject struct {
	ContentType string
	FileName    string
	SizeBytes   int64
	Data        io.ReadCloser
}
