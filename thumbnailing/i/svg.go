package i

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
)

type svgGenerator struct {
}

func (d svgGenerator) supportedContentTypes() []string {
	return []string{"image/svg+xml"}
}

func (d svgGenerator) supportsAnimation() bool {
	return false
}

func (d svgGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/svg+xml"
}

func (d svgGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d svgGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	dir, err := os.MkdirTemp(os.TempDir(), "mmr-svg")
	if err != nil {
		return nil, fmt.Errorf("svg: error creating temporary directory: %w", err)
	}

	tempFile1 := path.Join(dir, "i.svg")
	tempFile2 := path.Join(dir, "o.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	defer os.Remove(dir)

	f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		return nil, fmt.Errorf("svg: error creating temp svg file: %w", err)
	}
	if _, err = io.Copy(f, b); err != nil {
		return nil, fmt.Errorf("svg: error writing temp svg file: %w", err)
	}

	err = exec.Command("convert", tempFile1, tempFile2).Run()
	if err != nil {
		return nil, fmt.Errorf("svg: error converting svg file: %w", err)
	}

	f, err = os.OpenFile(tempFile2, os.O_RDONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("svg: error reading temp png file: %w", err)
	}
	defer f.Close()

	return pngGenerator{}.GenerateThumbnail(f, "image/png", width, height, method, false, ctx)
}

func init() {
	generators = append(generators, svgGenerator{})
}
