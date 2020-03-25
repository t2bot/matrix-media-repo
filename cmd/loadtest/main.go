package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/turt2live/matrix-media-repo/api/r0"
)

func main() {
	accessToken := flag.String("accessToken", "", "The access token to use to make requests to the localhost media repo")
	flag.Parse()

	uploadedMedia := make([]string, 0)
	lock := sync.RWMutex{}

	// Downloaders (thumbnail generation)
	numDownloaders := 45
	for x := 0; x < numDownloaders; x++ {
		go func() {
			for true {
				if len(uploadedMedia) == 0 {
					continue
				}
				i := int(math.Round(rand.Float64()*float64(len(uploadedMedia))))
				if i == len(uploadedMedia) {
					i--
				}
				mxc := uploadedMedia[i]
				if mxc == "" {
					continue
				}
				fmt.Println("Requesting media: ", mxc)
				resp, _ := http.Get(fmt.Sprintf("http://localhost:8001/_matrix/media/r0/thumbnail/%s?width=320&height=240&method=scale&animated=false", mxc[len("mxc://"):]))
				io.Copy(ioutil.Discard, resp.Body) // discard the body

				time.Sleep(time.Duration((rand.Float64() * 250) + 250) * time.Millisecond)
			}
		}()
	}

	// Uploaders
	numUploaders := 7
	for x := 0; x < numUploaders; x++ {
		go func() {
			for true {
				fmt.Println("Generating media...")

				// Generate a random image first
				width := int(math.Round(rand.Float64()*1024.0)) + 500
				height := int(math.Round(rand.Float64()*768.0) + 500)

				c := gg.NewContext(width, height)
				c.Clear()

				for x := 0; x < width; x++ {
					for y := 0; y < height; y++ {
						col := color.RGBA{
							R: uint8(math.Round(rand.Float64() * 255)),
							G: uint8(math.Round(rand.Float64() * 255)),
							B: uint8(math.Round(rand.Float64() * 255)),
							A: uint8(math.Round(rand.Float64() * 255)),
						}
						c.SetColor(col)
						c.SetPixel(x, y)
					}
				}

				buf := &bytes.Buffer{}
				c.EncodePNG(buf)

				// Now upload that image
				fmt.Println("Uploading media...")
				url := "http://localhost:8001/_matrix/media/r0/upload?filename=rand.png"
				req, _ := http.NewRequest("POST", url, buf)
				req.Header.Set("Content-Type", "image/png")
				req.Header.Set("Authorization", "Bearer "+*accessToken)
				resp, _ := http.DefaultClient.Do(req)
				jsonResp := r0.MediaUploadedResponse{}
				b, _ := ioutil.ReadAll(resp.Body)
				json.Unmarshal(b, &jsonResp)

				// Add that uploaded image to the known uris
				lock.Lock()
				uploadedMedia = append(uploadedMedia, jsonResp.ContentUri)
				fmt.Println("Added mxc uri: ", jsonResp.ContentUri)
				lock.Unlock()
			}
		}()
	}

	// wait forever (will need `kill -9` to kill this)
	c := make(chan bool)
	<-c
}
