package i

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
	"golang.org/x/image/webp"
)

type webpGenerator struct {
}

func (d webpGenerator) supportedContentTypes() []string {
	return []string{"image/webp"}
}

func (d webpGenerator) supportsAnimation() bool {
	return false
}

func (d webpGenerator) matches(img []byte, contentType string) bool {
	return contentType == "image/webp"
}

func (d webpGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := webp.Decode(bytes.NewBuffer(b))
	if err != nil {
		// the decoder isn't able to read all webp files. So, if it failed, we'll re-try with libwebp
		nativeDecodeError := err.Error()

		key, err := util.GenerateRandomString(16)
		if err != nil {
			return nil, errors.New("webp: error decoding thumbnail: " + nativeDecodeError + ", error generating temp key: " + err.Error())
		}
		tempFile1 := path.Join(os.TempDir(), "media_repo."+key+".1.webp")
		tempFile2 := path.Join(os.TempDir(), "media_repo."+key+".2.webp")
		tempFile3 := path.Join(os.TempDir(), "media_repo."+key+".3.png")
		defer os.Remove(tempFile1)
		defer os.Remove(tempFile2)
		defer os.Remove(tempFile3)

		f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0640)
		if err != nil {
			return nil, errors.New("webp: error decoding thumbnail: " + nativeDecodeError + ", error writing temp webp file: " + err.Error())
		}
		_, _ = f.Write(b)
		cleanup.DumpAndCloseStream(f)

		err = exec.Command("dwebp", tempFile1, "-o", tempFile3).Run()
		if err != nil {
			// the command failed, meaning the webp might have been animated. So, we
			// extrac tthe frame first and then try again
			err = exec.Command("webpmux", "-get", "frame", "1", tempFile1, "-o", tempFile2).Run()
			if err == nil {
				err = exec.Command("dwebp", tempFile2, "-o", tempFile3).Run()
			}
			if err != nil {
				// try via convert binary
				err = exec.Command("convert", tempFile1+"[0]", tempFile3).Run()
			}
			if err != nil {
				return nil, errors.New("webp: error decoding thumbnail: " + nativeDecodeError + ", error converting webp file: " + err.Error())
			}
		}

		b, err = ioutil.ReadFile(tempFile3)
		if err != nil {
			return nil, errors.New("webp: error decoding thumbnail: " + nativeDecodeError + ", error reading temp png file: " + err.Error())
		}

		return pngGenerator{}.GenerateThumbnail(b, "image/png", width, height, method, false, ctx)
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, webpGenerator{})
}
