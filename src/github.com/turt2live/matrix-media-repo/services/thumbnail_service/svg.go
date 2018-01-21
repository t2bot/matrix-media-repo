package thumbnail_service

import (
	"bytes"
	"image"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/types"
)

func (t *thumbnailer) svgToImage(media *types.Media) (image.Image, error) {
	tempFile := path.Join(os.TempDir(), "media_repo."+media.Origin+"."+media.MediaId+".png")
	defer os.Remove(tempFile)

	// requires imagemagick
	err := exec.Command("convert", media.Location, tempFile).Run()
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(tempFile)
	if err != nil {
		return nil, err
	}

	imgData := bytes.NewBuffer(b)
	return imaging.Decode(imgData)
}
