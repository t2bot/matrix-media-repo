package i

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type animatedWebpGenerator struct {
}

func (d animatedWebpGenerator) supportedContentTypes() []string {
	return []string{"image/webp"}
}

func (d animatedWebpGenerator) supportsAnimation() bool {
	return true
}

func (d animatedWebpGenerator) matches(img []byte, contentType string) bool {
	return contentType == "image/webp" && util.IsAnimatedWebp(img)
}

func (d animatedWebpGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	if !animated {
		return webpGenerator{}.GenerateThumbnail(b, "image/webp", width, height, method, false, ctx)
	}

	key, err := util.GenerateRandomString(16)
	if err != nil {
		return nil, errors.New("animated webp: error generating temp key: " + err.Error())
	}
	sourceFile := path.Join(os.TempDir(), "media_repo."+key+".1.webp")
	videoFile := path.Join(os.TempDir(), "media_repo."+key+".2.mp4")
	outFile := path.Join(os.TempDir(), "media_repo."+key+".3.webp")
	frameFile := path.Join(os.TempDir(), "media_repo."+key+".3.png")
	defer os.Remove(sourceFile)
	defer os.Remove(videoFile)
	defer os.Remove(outFile)
	defer os.Remove(frameFile)

	f, err := os.OpenFile(sourceFile, os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return nil, errors.New("animated webp: error writing temp webp file: " + err.Error())
	}
	_, _ = f.Write(b)
	cleanup.DumpAndCloseStream(f)

	// we need to fetch the first frame to be able to get source width / height
	err = exec.Command("convert", sourceFile+"[0]", frameFile).Run()
	if err != nil {
		return nil, errors.New("animated webp: error decoding webp first frame: " + err.Error())
	}
	b, err = ioutil.ReadFile(frameFile)
	if err != nil {
		return nil, errors.New("animated webp: error reading first frame: " + err.Error())
	}
	src, err := imaging.Decode(bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("animated webp: error decoding png first frame: " + err.Error())
	}

	var shouldThumbnail bool
	shouldThumbnail, width, height, _, method = u.AdjustProperties(src, width, height, false, false, method)
	if !shouldThumbnail {
		return nil, nil
	}

	err = exec.Command("convert", sourceFile, videoFile).Run()
	if err != nil {
		return nil, errors.New("animated webp: error converting webp file to mp4 file: " + err.Error())
	}

	srcWidth := src.Bounds().Max.X
	srcHeight := src.Bounds().Max.Y
	aspectRatio := float32(srcWidth) / float32(srcHeight)
	targetAspectRatio := float32(width) / float32(height)
	if method == "scale" {
		// ffmpeg -i out.mp4 -vf scale=width:heigth out.webp
		scaleWidth := width
		scaleHeight := height
		if targetAspectRatio < aspectRatio {
			// height needs increasing
			scaleHeight = int(float32(scaleWidth) * aspectRatio)
		} else {
			// width needs increasing
			scaleWidth = int(float32(scaleHeight) * aspectRatio)
		}
		err = exec.Command("ffmpeg", "-i", videoFile, "-vf", "scale=" + strconv.Itoa(scaleWidth) + ":" + strconv.Itoa(scaleHeight), outFile).Run()
	} else if method == "crop" {
		// ffmpeg -i out.mp4 -vf crop=iw-400:ih-40,scale=960:720 out.webp
		cropWidth := "iw"
		cropHeight := "ih"
		if targetAspectRatio < aspectRatio {
			// width needs cropping
			newWidth := float32(srcWidth) * targetAspectRatio / aspectRatio
			cropWidth = strconv.Itoa(int(newWidth))
		} else {
			// height needs cropping
			newHeight := float32(srcWidth) * aspectRatio / targetAspectRatio
			cropHeight = strconv.Itoa(int(newHeight))
		}
		err = exec.Command("ffmpeg", "-i", videoFile, "-vf", "crop=" + cropWidth + ":" + cropHeight + ",scale=" + strconv.Itoa(width) + ":" + strconv.Itoa(height), outFile).Run()
	} else {
		return nil, errors.New("animated webp: unrecognized method: " + method)
	}
	if err != nil {
		return nil, errors.New("animated webp: error scaling/cropping file: " + err.Error())
	}
	// set the animation to infinite loop again
	err = exec.Command("convert", outFile, "-loop", "0", outFile).Run()
	if err != nil {
		return nil, errors.New("animated webp: error setting webp to loop: " + err.Error())
	}
	b, err = ioutil.ReadFile(outFile)
	if err != nil {
		return nil, errors.New("animated webp: error reading resulting webp thumbnail: " + err.Error())
	}
	return &m.Thumbnail{
		Animated:    true,
		ContentType: "image/webp",
		Reader:      ioutil.NopCloser(bytes.NewReader(b)),
	}, nil
}

func init() {
	generators = append(generators, animatedWebpGenerator{})
}
