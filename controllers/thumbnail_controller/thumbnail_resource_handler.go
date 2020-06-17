package thumbnail_controller

import (
	"bytes"
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
	"github.com/kettek/apng"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
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
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{
		"worker_requestId": request.Id,
		"worker_media":     info.media.Origin + "/" + info.media.MediaId,
		"worker_width":     info.width,
		"worker_height":    info.height,
		"worker_method":    info.method,
		"worker_animated":  info.animated,
	})
	ctx.Log.Info("Processing thumbnail request")

	generated, err := GenerateThumbnail(info.media, info.width, info.height, info.method, info.animated, ctx)
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

	db := storage.GetDatabase().GetThumbnailStore(ctx)
	err = db.Insert(newThumb)
	if err != nil {
		ctx.Log.Error("Unexpected error caching thumbnail: " + err.Error())
		return &thumbnailResponse{err: err}
	}

	return &thumbnailResponse{thumbnail: newThumb}
}

func (h *thumbnailResourceHandler) GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool) chan *thumbnailResponse {
	resultChan := make(chan *thumbnailResponse)
	go func() {
		reqId := fmt.Sprintf("thumbnail_%s_%s_%d_%d_%s_%t", media.Origin, media.MediaId, width, height, method, animated)
		c := h.resourceHandler.GetResource(reqId, &thumbnailRequest{
			media:    media,
			width:    width,
			height:   height,
			method:   method,
			animated: animated,
		})
		defer close(c)
		result := <-c
		resultChan <- result.(*thumbnailResponse)
	}()
	return resultChan
}

func GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*GeneratedThumbnail, error) {
	var src image.Image
	var err error

	canAnimate := util.ArrayContains(animatedTypes, media.ContentType)
	allowAnimated := ctx.Config.Thumbnails.AllowAnimated

	if media.ContentType == "image/svg+xml" {
		src, err = svgToImage(media, ctx)
	} else if canAnimate && !animated {
		src, err = pickImageFrame(media, ctx)
	} else {
		mediaStream, err2 := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
		if err2 != nil {
			ctx.Log.Error("Error getting file: ", err2)
			return nil, err2
		}
		defer cleanup.DumpAndCloseStream(mediaStream)
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
		ctx.Log.Info("Aspect ratio is the same, converting method to 'scale'")
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
			ctx.Log.Warn("Image is too small but the image should be animated. Adjusting dimensions to fit image exactly.")
			width = srcWidth
			height = srcHeight
		} else if canAnimate && !animated {
			ctx.Log.Warn("Image is too small, but the request calls for a static image. Adjusting dimensions to fit image exactly.")
			width = srcWidth
			height = srcHeight
		} else {
			// Image is too small - don't upscale
			thumb.ContentType = media.ContentType
			thumb.DatastoreId = media.DatastoreId
			thumb.DatastoreLocation = media.Location
			thumb.SizeBytes = media.SizeBytes
			thumb.Sha256Hash = media.Sha256Hash
			ctx.Log.Warn("Image too small, returning raw image")
			metric.Inc()
			return thumb, nil
		}
	}

	var orientation *util_exif.ExifOrientation = nil
	if media.ContentType == "image/jpeg" || media.ContentType == "image/jpg" {
		orientation, err = util_exif.GetExifOrientation(media)
		if err != nil {
			ctx.Log.Warn("Non-fatal error getting EXIF orientation: " + err.Error())
			orientation = nil // just in case
		}
	}

	contentType := "image/png"
	imgData := &bytes.Buffer{}
	if allowAnimated && animated && media.ContentType == "image/gif" {
		ctx.Log.Info("Generating animated gif thumbnail")
		contentType = "image/gif"

		// Animated GIFs are a bit more special because we need to do it frame by frame.
		// This is fairly resource intensive. The calling code is responsible for limiting this case.

		mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
		if err != nil {
			ctx.Log.Error("Error resolving datastore path: ", err)
			return nil, err
		}
		defer cleanup.DumpAndCloseStream(mediaStream)

		g, err := gif.DecodeAll(mediaStream)
		if err != nil {
			ctx.Log.Error("Error generating animated thumbnail: " + err.Error())
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
				ctx.Log.Error("Error generating animated thumbnail frame: " + err.Error())
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
			ctx.Log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}
	} else if allowAnimated && animated && media.ContentType == "image/png" {
		ctx.Log.Info("Generating animated png thumbnail")
		contentType = "image/png"

		// scale animated pngs frame by frame

		mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
		if err != nil {
			ctx.Log.Error("Error resolving datastore path: ", err)
			return nil, err
		}
		defer cleanup.DumpAndCloseStream(mediaStream)

		p, err := apng.DecodeAll(mediaStream)
		if err != nil {
			ctx.Log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}

		// prepare a blank frame to use as swap space
		frameImg := image.NewRGBA(p.Frames[0].Image.Bounds())

		widthRatio := float64(width) / float64(p.Frames[0].Image.Bounds().Dx())
		heightRatio := float64(width) / float64(p.Frames[0].Image.Bounds().Dy())
		ctx.Log.Warn("widthRatio", widthRatio, "heightRatio", heightRatio);

		for i := range p.Frames {
			frame := p.Frames[i]
			img := frame.Image

			// Clear the transparency of the previous frame
			if frame.DisposeOp == apng.DISPOSE_OP_NONE {
				frame.DisposeOp = apng.DISPOSE_OP_BACKGROUND
			} else {
				draw.Draw(frameImg, frameImg.Bounds(), image.Transparent, image.ZP, draw.Src)
			}

			// Copy the frame to a new image and use that
			draw.Draw(frameImg, image.Rectangle{image.Point{frame.XOffset, frame.YOffset}, image.Point{img.Bounds().Dx(), img.Bounds().Dy()}}, img, image.ZP, draw.Over)

			// Do the thumbnailing on the copied frame
			frameThumb, err := thumbnailFrame(frameImg, method, width, height, imaging.Linear, nil)
			if err != nil {
				ctx.Log.Error("Error generating animated thumbnail frame: " + err.Error())
				return nil, err
			}
			p.Frames[i].Image = frameThumb
			newXOffset := int(math.Floor(float64(frame.XOffset) * widthRatio))
			newYOffset := int(math.Floor(float64(frame.YOffset) * heightRatio))
			// we need to make sure that these are still in the image bounds
			if p.Frames[0].Image.Bounds().Dx() <= newXOffset + frameThumb.Bounds().Dx() {
				newXOffset = p.Frames[0].Image.Bounds().Dx() - frameThumb.Bounds().Dx()
			}
			if p.Frames[0].Image.Bounds().Dy() <= newYOffset + frameThumb.Bounds().Dy() {
				newYOffset = p.Frames[0].Image.Bounds().Dy() - frameThumb.Bounds().Dy()
			}
			p.Frames[i].XOffset = newXOffset
			p.Frames[i].YOffset = newYOffset
		}
		err = apng.Encode(imgData, p)
		if err != nil {
			ctx.Log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}
	} else {
		src, err = thumbnailFrame(src, method, width, height, imaging.Linear, orientation)
		if err != nil {
			ctx.Log.Error("Error generating thumbnail: " + err.Error())
			return nil, err
		}

		// Put the image bytes into a memory buffer
		err = imaging.Encode(imgData, src, imaging.PNG)
		if err != nil {
			ctx.Log.Error("Unexpected error encoding thumbnail: " + err.Error())
			return nil, err
		}
	}

	// Reset the buffer pointer and store the file
	ds, err := datastore.PickDatastore(common.KindThumbnails, ctx)
	if err != nil {
		return nil, err
	}
	info, err := ds.UploadFile(util.BufferToStream(imgData), int64(len(imgData.Bytes())), ctx)
	if err != nil {
		ctx.Log.Error("Unexpected error saving thumbnail: " + err.Error())
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

func svgToImage(media *types.Media, ctx rcontext.RequestContext) (image.Image, error) {
	tempFile1 := path.Join(os.TempDir(), "media_repo."+media.Origin+"."+media.MediaId+".1.png")
	tempFile2 := path.Join(os.TempDir(), "media_repo."+media.Origin+"."+media.MediaId+".2.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)

	// requires imagemagick
	mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
	if err != nil {
		ctx.Log.Error("Error streaming file: ", err)
		return nil, err
	}
	defer cleanup.DumpAndCloseStream(mediaStream)

	f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return nil, err
	}
	io.Copy(f, mediaStream)
	cleanup.DumpAndCloseStream(f)

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

func pickImageFrame(media *types.Media, ctx rcontext.RequestContext) (image.Image, error) {
	mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
	if err != nil {
		ctx.Log.Error("Error resolving datastore path: ", err)
		return nil, err
	}
	defer cleanup.DumpAndCloseStream(mediaStream)

	stillFrameRatio := float64(ctx.Config.Thumbnails.StillFrame)
	getFrameIndex := func (numFrames int) int {
		frameIndex := int(math.Floor(math.Min(1, math.Max(0, stillFrameRatio)) * float64(numFrames)))
		ctx.Log.Info("Picking frame ", frameIndex, " for animated file")
		return frameIndex
	}

	if media.ContentType == "image/gif" {
		g, err := gif.DecodeAll(mediaStream)
		if err != nil {
			ctx.Log.Error("Error picking frame: " + err.Error())
			return nil, err
		}

		frameIndex := getFrameIndex(len(g.Image))
		return g.Image[frameIndex], nil
	}
	if media.ContentType == "image/png" {
		p, err := apng.DecodeAll(mediaStream)
		if err != nil {
			ctx.Log.Error("Error picking frame: " + err.Error())
			return nil, err
		}

		frameIndex := getFrameIndex(len(p.Frames))
		return p.Frames[frameIndex].Image, nil
	}
	return nil, errors.New("Unknown animation type: " + media.ContentType)
}
