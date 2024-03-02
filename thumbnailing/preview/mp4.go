package preview

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
)

type mp4Generator struct{}

func (d mp4Generator) supportedContentTypes() []string {
	return []string{"video/mp4"}
}

func (d mp4Generator) supportsAnimation() bool {
	return false
}

func (d mp4Generator) matches(img io.Reader, contentType string) bool {
	return slices.Contains(d.supportedContentTypes(), contentType)
}

func (d mp4Generator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d mp4Generator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	dir, err := os.MkdirTemp(os.TempDir(), "mmr-mp4")
	if err != nil {
		return nil, fmt.Errorf("mp4: error creating temporary directory: %w", err)
	}

	tempFile1 := path.Join(dir, "i.mp4")
	tempFile2 := path.Join(dir, "o.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	defer os.Remove(dir)

	f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		return nil, fmt.Errorf("mp4: error creating temp video file: %w", err)
	}
	if _, err = io.Copy(f, b); err != nil {
		return nil, fmt.Errorf("mp4: error writing temp video file: %w", err)
	}

	err = exec.Command("ffmpeg", "-i", tempFile1, "-vf", "select=eq(n\\,0)", tempFile2).Run()
	if err != nil {
		return nil, fmt.Errorf("mp4: error converting video file: %w", err)
	}

	f, err = os.OpenFile(tempFile2, os.O_RDONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("mp4: error reading temp png file: %w", err)
	}
	defer f.Close()

	return pngGenerator{}.GenerateThumbnail(f, "image/png", width, height, method, false, ctx)
}

func init() {
	generators = append(generators, mp4Generator{})
}
