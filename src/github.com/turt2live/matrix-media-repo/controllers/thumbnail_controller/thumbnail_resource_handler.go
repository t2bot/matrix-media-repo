package thumbnail_controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/resource_handler"
	"github.com/turt2live/matrix-media-repo/util/util_exif"
)

type thumbnailResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type thumbnailRequest struct {
	media    *types.Media
	width    int
	height   int
	method   string
	animated bool
}

type thumbnailResponse struct {
	thumbnail *types.Thumbnail
	err       error
}

type GeneratedThumbnail struct {
	ContentType       string
	DatastoreId       string
	DatastoreLocation string
	SizeBytes         int64
	Animated          bool
	Sha256Hash        string
}

var resHandlerInstance *thumbnailResourceHandler
var resHandlerSingletonLock = &sync.Once{}

func getResourceHandler() *thumbnailResourceHandler {
	if resHandlerInstance == nil {
		resHandlerSingletonLock.Do(func() {
			handler, err := resource_handler.New(config.Get().Thumbnails.NumWorkers, thumbnailWorkFn)
			if err != nil {
				panic(err)
			}

			resHandlerInstance = &thumbnailResourceHandler{handler}
		})
	}

	return resHandlerInstance
}

func thumbnailWorkFn(request *resource_handler.WorkRequest) interface{} {
	info := request.Metadata.(*thumbnailRequest)
	log := logrus.WithFields(logrus.Fields{
		"worker_requestId": request.Id,
		"worker_media":     info.media.Origin + "/" + info.media.MediaId,
		"worker_width":     info.width,
		"worker_height":    info.height,
		"worker_method":    info.method,
		"worker_animated":  info.animated,
	})
	log.Info("Processing thumbnail request")

	ctx := context.TODO() // TODO: Should we use a real context?

	generated, err := GenerateThumbnail(info.media, info.width, info.height, info.method, info.animated, ctx, log)
	if err != nil {
		return &thumbnailResponse{err: err}
	}

	newThumb := &types.Thumbnail{
		Origin:      info.media.Origin,
		MediaId:     info.media.MediaId,
		Width:       info.width,
		Height:      info.height,
		Method:      info.method,
		Animated:    generated.Animated,
		CreationTs:  util.NowMillis(),
		ContentType: generated.ContentType,
		DatastoreId: generated.DatastoreId,
		Location:    generated.DatastoreLocation,
		SizeBytes:   generated.SizeBytes,
		Sha256Hash:  generated.Sha256Hash,
	}

	db := storage.GetDatabase().GetThumbnailStore(ctx, log)
	err = db.Insert(newThumb)
	if err != nil {
		log.Error("Unexpected error caching thumbnail: " + err.Error())
		return &thumbnailResponse{err: err}
	}

	return &thumbnailResponse{thumbnail: newThumb}
}

func (h *thumbnailResourceHandler) GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool) chan *thumbnailResponse {
	resultChan := make(chan *thumbnailResponse)
	go func() {
		reqId := fmt.Sprintf("thumbnail_%s_%s_%d_%d_%s_%t", media.Origin, media.MediaId, width, height, method, animated)
		result := <-h.resourceHandler.GetResource(reqId, &thumbnailRequest{
			media:    media,
			width:    width,
			height:   height,
			method:   method,
			animated: animated,
		})
		resultChan <- result.(*thumbnailResponse)
	}()
	return resultChan
}

func GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool, ctx context.Context, log *logrus.Entry) (*GeneratedThumbnail, error) {
	var src image.Image
	var err error

	canAnimate := util.ArrayContains(animatedTypes, media.ContentType)
	allowAnimated := config.Get().Thumbnails.AllowAnimated

	if media.ContentType == "image/svg+xml" {
		src, err = svgToImage(media, ctx, log)
	} else if canAnimate && !animated {
		src, err = pickImageFrame(media, ctx, log)
	} else {
		mediaStream, err2 := datastore.DownloadStream(ctx, log, media.DatastoreId, media.Location)
		if err2 != nil {
			log.Error("Error getting file: ", err2)
			return nil, err2
		}
		src, err = imaging.Decode(mediaStream)
	}

	if err != nil {
		return nil, err
	}

	srcWidth := src.Bounds().Max.X
	srcHeight := src.Bounds().Max.Y

	aspectRatio := float32(srcHeight) / float32(srcWidth)
	targetAspectRatio := float32(width) / float32(height)
	if aspectRatio == targetAspectRatio {
		// Highly unlikely, but if the aspect ratios match then just resize
		method = "scale"
		log.Info("Aspect ratio is the same, converting method to 'scale'")
	}

	metric := metrics.ThumbnailsGenerated.With(prometheus.Labels{
		"width":    strconv.Itoa(width),
		"height":   strconv.Itoa(height),
		"method":   method,
		"animated": strconv.FormatBool(animated),
		"origin":   media.Origin,
	})

	thumb := &GeneratedThumbnail{
		Animated: animated,
	}

	if srcWidth <= width && srcHeight <= height {
		if animated {
			log.Warn("Image is too small but the image should be animated. Adjusting dimensions to fit image exactly.")
			width = srcWidth
			height = srcHeight
		} else if canAnimate && !animated {
			log.Warn("Image is too small, but the request calls for a static image. Adjusting dimensions to fit image exactly.")
			width = srcWidth
			height = srcHeight
		} else {
			// Image is too small - don't upscale
			thumb.ContentType = media.ContentType
			thumb.DatastoreId = media.DatastoreId
			thumb.DatastoreLocation = media.Location
			thumb.SizeBytes = media.SizeBytes
			thumb.Sha256Hash = media.Sha256Hash
			log.Warn("Image too small, returning raw image")
			metric.Inc()
			return thumb, nil
		}
	}

	var orientation *util_exif.ExifOrientation = nil
	if media.ContentType == "image/jpeg" || media.ContentType == "image/jpg" {
		orientation, err = util_exif.GetExifOrientation(media)
		if err != nil {
			log.Warn("Non-fatal error getting EXIF orientation: " + err.Error())
			orientation = nil // just in case
		}
	}

	contentType := "image/png"
	imgData := &bytes.Buffer{}
	if allowAnimated && animated {
		log.Info("Generating animated thumbnail")
		contentType = "image/gif"

		// Animated GIFs are a bit more special because we need to do it frame by frame.
		// This is fairly resource intensive. The calling code is responsible for limiting this case.

		mediaStream, err := datastore.DownloadStream(ctx, log, media.DatastoreId, media.Location)
		if err != nil {
			log.Error("Error resolving datastore path: ", err)
			return nil, err
		}

		g, err := gif.DecodeAll(mediaStream)
		if err != nil {
			log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}

		// Prepare a blank frame to use as swap space
		frameImg := image.NewRGBA(g.Image[0].Bounds())

		for i := range g.Image {
			img := g.Image[i]

			// Clear the transparency of the previous frame
			draw.Draw(frameImg, frameImg.Bounds(), image.Transparent, image.ZP, draw.Src)

			// Copy the frame to a new image and use that
			draw.Draw(frameImg, frameImg.Bounds(), img, image.ZP, draw.Over)

			// Do the thumbnailing on the copied frame
			frameThumb, err := thumbnailFrame(frameImg, method, width, height, imaging.Linear, nil)
			if err != nil {
				log.Error("Error generating animated thumbnail frame: " + err.Error())
				return nil, err
			}

			//t.log.Info(fmt.Sprintf("Width = %d    Height = %d    FW=%d    FH=%d", width, height, frameThumb.Bounds().Max.X, frameThumb.Bounds().Max.Y))

			targetImg := image.NewPaletted(frameThumb.Bounds(), img.Palette)
			draw.FloydSteinberg.Draw(targetImg, frameThumb.Bounds(), frameThumb, image.ZP)
			g.Image[i] = targetImg
		}

		// Set the image size to the first frame's size
		g.Config.Width = g.Image[0].Bounds().Max.X
		g.Config.Height = g.Image[0].Bounds().Max.Y

		err = gif.EncodeAll(imgData, g)
		if err != nil {
			log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}
	} else {
		src, err = thumbnailFrame(src, method, width, height, imaging.Lanczos, orientation)
		if err != nil {
			log.Error("Error generating thumbnail: " + err.Error())
			return nil, err
		}

		// Put the image bytes into a memory buffer
		err = imaging.Encode(imgData, src, imaging.PNG)
		if err != nil {
			log.Error("Unexpected error encoding thumbnail: " + err.Error())
			return nil, err
		}
	}

	// Reset the buffer pointer and store the file
	ds, err := datastore.PickDatastore(ctx, log)
	if err != nil {
		return nil, err
	}
	info, err := ds.UploadFile(util.BufferToStream(imgData), ctx, log)
	if err != nil {
		log.Error("Unexpected error saving thumbnail: " + err.Error())
		return nil, err
	}

	thumb.DatastoreLocation = info.Location
	thumb.DatastoreId = ds.DatastoreId
	thumb.ContentType = contentType
	thumb.SizeBytes = info.SizeBytes
	thumb.Sha256Hash = info.Sha256Hash

	metric.Inc()
	return thumb, nil
}

func thumbnailFrame(src image.Image, method string, width int, height int, filter imaging.ResampleFilter, orientation *util_exif.ExifOrientation) (image.Image, error) {
	var result image.Image
	if method == "scale" {
		result = imaging.Fit(src, width, height, filter)
	} else if method == "crop" {
		result = imaging.Fill(src, width, height, imaging.Center, filter)
	} else {
		return nil, errors.New("unrecognized method: " + method)
	}

	if orientation != nil {
		// Rotate first
		if orientation.RotateDegrees == 90 {
			result = imaging.Rotate90(result)
		} else if orientation.RotateDegrees == 180 {
			result = imaging.Rotate180(result)
		} else if orientation.RotateDegrees == 270 {
			result = imaging.Rotate270(result)
		} // else we don't care to rotate

		// Flip second
		if orientation.FlipHorizontal {
			result = imaging.FlipH(result)
		}
		if orientation.FlipVertical {
			result = imaging.FlipV(result)
		}
	}

	return result, nil
}

func svgToImage(media *types.Media, ctx context.Context, log *logrus.Entry) (image.Image, error) {
	tempFile1 := path.Join(os.TempDir(), "media_repo."+media.Origin+"."+media.MediaId+".1.png")
	tempFile2 := path.Join(os.TempDir(), "media_repo."+media.Origin+"."+media.MediaId+".2.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)

	// requires imagemagick
	mediaStream, err := datastore.DownloadStream(ctx, log, media.DatastoreId, media.Location)
	if err != nil {
		log.Error("Error streaming file: ", err)
		return nil, err
	}

	f, err := os.Open(tempFile1)
	if err != nil {
		return nil, err
	}
	io.Copy(f, mediaStream)
	f.Close()

	err = exec.Command("convert", tempFile1, tempFile2).Run()
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(tempFile2)
	if err != nil {
		return nil, err
	}

	imgData := bytes.NewBuffer(b)
	return imaging.Decode(imgData)
}

func pickImageFrame(media *types.Media, ctx context.Context, log *logrus.Entry) (image.Image, error) {
	mediaStream, err := datastore.DownloadStream(ctx, log, media.DatastoreId, media.Location)
	if err != nil {
		log.Error("Error resolving datastore path: ", err)
		return nil, err
	}

	g, err := gif.DecodeAll(mediaStream)
	if err != nil {
		log.Error("Error picking frame: " + err.Error())
		return nil, err
	}

	stillFrameRatio := float64(config.Get().Thumbnails.StillFrame)
	frameIndex := int(math.Floor(math.Min(1, math.Max(0, stillFrameRatio)) * float64(len(g.Image))))
	log.Info("Picking frame ", frameIndex, " for animated file")

	return g.Image[frameIndex], nil
}
