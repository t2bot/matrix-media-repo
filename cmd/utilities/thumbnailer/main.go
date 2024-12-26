package main

import (
	"flag"
	"io"
	"os"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/logging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing"
	"github.com/t2bot/matrix-media-repo/util"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	inFile := flag.String("i", "", "The input file to thumbnail")
	outFile := flag.String("o", "", "The output file to write the thumbnail to. Note that MMR chooses the output file format and won't inspect the extension type supplied here. Typically, output format is PNG.")
	targetWidth := flag.Int("w", 512, "The target width of the thumbnail")
	targetHeight := flag.Int("h", 512, "The target height of the thumbnail")
	targetMethod := flag.String("m", "scale", "The method to use for resizing. Can be 'scale' or 'crop'")
	targetAnimated := flag.Bool("a", false, "Whether the thumbnail should be animated (if supported)")
	forceMime := flag.String("f", "", "Force the mime type of the input file, ignoring the detected mime type")
	flag.Parse()

	if inFile == nil || *inFile == "" {
		panic("No input file specified")
	}
	if outFile == nil || *outFile == "" {
		panic("No output file specified")
	}

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Runtime.IsImportProcess = true // prevents us from creating media by accident
	config.Path = *configPath

	var err error
	err = logging.Setup(
		config.Get().General.LogDirectory,
		config.Get().General.LogColors,
		config.Get().General.JsonLogs,
		config.Get().General.LogLevel,
	)
	if err != nil {
		panic(err)
	}
	ctx := rcontext.Initial()

	ctx.Log.WithField("width", *targetWidth).WithField("height", *targetHeight).WithField("method", *targetMethod).WithField("animated", *targetAnimated).Info("Thumbnailing options:")

	// Read source image
	f, err := os.Open(*inFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	mime, err := util.DetectMimeType(f)
	if err != nil {
		panic(err)
	}
	ctx.Log.WithField("mime", mime).Info("Detected mime type")

	if forceMime != nil && *forceMime != "" {
		mime = *forceMime
		ctx.Log.WithField("mime", mime).Warn("Forcing mime type")
	}

	ctx.Log.Info("Generating thumbnail")
	t, err := thumbnailing.GenerateThumbnail(f, mime, *targetWidth, *targetHeight, *targetMethod, *targetAnimated, ctx)
	if err != nil {
		panic(err)
	}

	ctx.Log.WithField("animated", t.Animated).WithField("contentType", t.ContentType).Info("Writing generated thumbnail")

	f2, err := os.Create(*outFile)
	if err != nil {
		panic(err)
	}
	defer f2.Close()
	if _, err = io.Copy(f2, t.Reader); err != nil {
		panic(err)
	}

	ctx.Log.Info("Done!")
}
