package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/controllers/data_controller"
	"github.com/turt2live/matrix-media-repo/synapse"
	"github.com/turt2live/matrix-media-repo/templating"
	"github.com/turt2live/matrix-media-repo/util"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	postgresHost := flag.String("dbHost", "localhost", "The PostgresSQL hostname for your Synapse database")
	postgresPort := flag.Int("dbPort", 5432, "The port for your Synapse's PostgreSQL database")
	postgresUsername := flag.String("dbUsername", "synapse", "The username for your Synapse's PostgreSQL database")
	postgresPassword := flag.String("dbPassword", "", "The password for your Synapse's PostgreSQL database. Can be omitted to be prompted when run")
	postgresDatabase := flag.String("dbName", "synapse", "The name of your Synapse database")
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org)")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the templates folder")
	exportPath := flag.String("destination", "./media-export", "The directory to export the files to (will be created if needed)")
	importPath := flag.String("mediaDirectory", "./media_store", "The media_store_path for Synapse")
	partSizeBytes := flag.Int64("partSize", 104857600, "The number of bytes (roughly) to split the export files into.")
	flag.Parse()

	assets.SetupTemplates(*templatesPath)

	_ = os.MkdirAll(*exportPath, 0755)

	var realPsqlPassword string
	if *postgresPassword == "" {
		if !terminal.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println("Sorry, your terminal does not support reading passwords. Please supply a -dbPassword or use a different terminal.")
			fmt.Println("If you're on Windows, try using a plain Command Prompt window instead of a bash-like terminal.")
			os.Exit(1)
			return // for good measure
		}
		fmt.Printf("Postgres password: ")
		pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		realPsqlPassword = string(pass[:])
	} else {
		realPsqlPassword = *postgresPassword
	}

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Setting up for importing...")

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"

	logrus.Info("Connecting to synapse database...")
	synDb, err := synapse.OpenDatabase(connectionString)
	if err != nil {
		panic(err)
	}

	logrus.Info("Fetching all local media records from synapse...")
	records, err := synDb.GetAllMedia()
	if err != nil {
		panic(err)
	}

	logrus.Info(fmt.Sprintf("Exporting %d media records", len(records)))

	// TODO: Share this logic with export_controller somehow
	var currentTar *tar.Writer
	var currentTarBytes bytes.Buffer
	part := 0
	currentSize := int64(0)
	isManifestTar := false

	persistTar := func() error {
		_ = currentTar.Close()

		// compress
		logrus.Info("Compressing tar file...")
		gzipBytes := bytes.Buffer{}
		archiver := gzip.NewWriter(&gzipBytes)
		archiver.Name = fmt.Sprintf("export-part-%d.tar", part)
		if isManifestTar {
			archiver.Name = fmt.Sprintf("export-manifest.tar")
		}
		_, err := io.Copy(archiver, bytes.NewBuffer(currentTarBytes.Bytes()))
		if err != nil {
			return err
		}
		_ = archiver.Close()

		logrus.Info("Writing compressed tar to disk...")
		name := fmt.Sprintf("export-part-%d.tgz", part)
		if isManifestTar {
			name = "export-manifest.tgz"
		}
		f, err := os.Create(path.Join(*exportPath, name))
		if err != nil {
			return err
		}
		_, _ = io.Copy(f, &gzipBytes)
		_ = f.Close()

		return nil
	}

	newTar := func() error {
		if part > 0 {
			logrus.Info("Persisting complete tar file...")
			err := persistTar()
			if err != nil {
				return err
			}
		}

		logrus.Info("Starting new tar file...")
		currentTarBytes = bytes.Buffer{}
		currentTar = tar.NewWriter(&currentTarBytes)
		part = part + 1
		currentSize = 0

		return nil
	}

	// Start the first tar file
	logrus.Info("Preparing first tar file...")
	err = newTar()
	if err != nil {
		logrus.Fatal(err)
	}

	putFile := func(name string, size int64, creationTime time.Time, file io.Reader) error {
		header := &tar.Header{
			Name:    name,
			Size:    size,
			Mode:    int64(0644),
			ModTime: creationTime,
		}
		err := currentTar.WriteHeader(header)
		if err != nil {
			return err
		}

		i, err := io.Copy(currentTar, file)
		if err != nil {
			return err
		}

		currentSize += i

		return nil
	}

	archivedName := func(origin string, mediaId string) string {
		// TODO: Pick the right extension for the file type
		return fmt.Sprintf("%s__%s.obj", origin, mediaId)
	}

	logrus.Info("Preparing manifest...")
	indexModel := &templating.ExportIndexModel{
		Entity:   *serverName,
		ExportID: "OOB",
		Media:    make([]*templating.ExportIndexMediaModel, 0),
	}
	mediaManifest := make(map[string]*data_controller.ManifestRecord)

	for _, r := range records {
		// For MediaID AABBCCDD :
		// $importPath/local_content/AA/BB/CCDD
		//
		// For a URL MediaID 2020-08-17_AABBCCDD:
		// $importPath/url_cache/2020-08-17/AABBCCDD

		mxc := fmt.Sprintf("mxc://%s/%s", *serverName, r.MediaId)

		logrus.Info("Copying " + mxc)

		filePath := path.Join(*importPath, "local_content", r.MediaId[0:2], r.MediaId[2:4], r.MediaId[4:])
		if r.UrlCache != "" {
			dateParts := strings.Split(r.MediaId, "_")
			filePath = path.Join(*importPath, "url_cache", dateParts[0], strings.Join(dateParts[1:], "_"))
		}

		f, err := os.Open(filePath)
		if err != nil {
			logrus.Fatal(err)
		}

		d := &bytes.Buffer{}
		_, _ = io.Copy(d, f)
		_ = f.Close()

		temp := bytes.NewBuffer(d.Bytes())
		sha256, err := util.GetSha256HashOfStream(ioutil.NopCloser(temp))
		if err != nil {
			logrus.Fatal(err)
		}

		err = putFile(archivedName(*serverName, r.MediaId), r.SizeBytes, util.FromMillis(r.CreatedTs), d)
		if err != nil {
			logrus.Fatal(err)
		}

		if currentSize >= *partSizeBytes {
			logrus.Info("Rotating tar...")
			err = newTar()
			if err != nil {
				logrus.Fatal(err)
			}
		}

		mediaManifest[mxc] = &data_controller.ManifestRecord{
			ArchivedName: archivedName(*serverName, r.MediaId),
			FileName:     r.UploadName,
			SizeBytes:    r.SizeBytes,
			ContentType:  r.ContentType,
			S3Url:        "",
			Sha256:       sha256,
			Origin:       *serverName,
			MediaId:      r.MediaId,
			CreatedTs:    r.CreatedTs,
			Uploader:     r.UserId,
		}
		indexModel.Media = append(indexModel.Media, &templating.ExportIndexMediaModel{
			ExportID:        "OOB",
			ArchivedName:    archivedName(*serverName, r.MediaId),
			FileName:        r.UploadName,
			SizeBytes:       r.SizeBytes,
			SizeBytesHuman:  humanize.Bytes(uint64(r.SizeBytes)),
			Origin:          *serverName,
			MediaID:         r.MediaId,
			Sha256Hash:      sha256,
			ContentType:     r.ContentType,
			UploadTs:        r.CreatedTs,
			UploadDateHuman: util.FromMillis(r.CreatedTs).Format(time.UnixDate),
			Uploader:        r.UserId,
		})
	}

	logrus.Info("Preparing manifest-specific tar...")
	err = newTar()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Writing manifest...")
	isManifestTar = true
	manifest := &data_controller.Manifest{
		Version:   2,
		EntityId:  *serverName,
		CreatedTs: util.NowMillis(),
		Media:     mediaManifest,
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		logrus.Fatal(err)
	}
	err = putFile("manifest.json", int64(len(b)), time.Now(), bytes.NewBuffer(b))
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Building and writing index...")
	t, err := templating.GetTemplate("export_index")
	if err != nil {
		logrus.Fatal(err)
		return
	}
	html := bytes.Buffer{}
	err = t.Execute(&html, indexModel)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	err = putFile("index.html", int64(html.Len()), time.Now(), util.BufferToStream(bytes.NewBuffer(html.Bytes())))
	if err != nil {
		logrus.Fatal(err)
		return
	}

	logrus.Info("Writing final tar...")
	err = persistTar()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Done export - cleaning up...")

	// Clean up
	assets.Cleanup()

	logrus.Info("Import completed")
}
