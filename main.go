package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/discordapp/lilliput"
)

var addr string
var baseURL string

var EncodeOptions = map[string]map[int]int{
	".jpeg": map[int]int{lilliput.JpegQuality: 85},
	".png":  map[int]int{lilliput.PngCompression: 7},
	".webp": map[int]int{lilliput.WebpQuality: 85},
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	var imagePath = r.URL.Path
	var outputWidth int
	var outputHeight int
	var stretch = true

	var client = &http.Client{
		Timeout: time.Second * 10,
	}
	client.Transport = http.DefaultTransport

	fmt.Println("Image path was:", baseURL+imagePath)
	req, err := http.NewRequest("GET", baseURL+imagePath, nil)
	resp, err := client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("invalid request URL: %v", err)
		log.Print(msg)

		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	fmt.Println("GET params were:", r.URL.Query())
	outputWidth, err = strconv.Atoi(r.URL.Query().Get("w"))
	if err != nil {
		msg := fmt.Sprintf("error parsing width, %s\n", err)
		log.Print(msg)

		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	outputHeight, err = strconv.Atoi(r.URL.Query().Get("h"))
	if err != nil {
		msg := fmt.Sprintf("error parsing height, %s\n", err)
		log.Print(msg)

		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	inputBuf, err := ioutil.ReadAll(resp.Body)
	decoder, err := lilliput.NewDecoder(inputBuf)
	// this error reflects very basic checks,
	// mostly just for the magic bytes of the file to match known image formats
	if err != nil {
		msg := fmt.Sprintf("error decoding image, %s\n", err)
		log.Print(msg)

		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	defer decoder.Close()

	header, err := decoder.Header()
	// this error is much more comprehensive and reflects
	// format errors
	if err != nil {
		msg := fmt.Sprintf("error reading image header, %s\n", err)
		log.Print(msg)

		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	// print some basic info about the image
	fmt.Printf("image type: %s\n", decoder.Description())
	fmt.Printf("%dpx x %dpx\n", header.Width(), header.Height())

	// get ready to resize image,
	// using 8192x8192 maximum resize buffer size
	ops := lilliput.NewImageOps(8192)
	defer ops.Close()

	// create a buffer to store the output image, 50MB in this case
	outputImg := make([]byte, 50*1024*1024)

	// use user supplied filename to guess output type if provided
	// otherwise don't transcode (use existing type)
	outputType := "." + strings.ToLower(decoder.Description())

	if outputWidth == 0 {
		outputWidth = header.Width()
	}

	if outputHeight == 0 {
		outputHeight = header.Height()
	}

	resizeMethod := lilliput.ImageOpsFit
	if stretch {
		resizeMethod = lilliput.ImageOpsResize
	}

	opts := &lilliput.ImageOptions{
		FileType:             outputType,
		Width:                outputWidth,
		Height:               outputHeight,
		ResizeMethod:         resizeMethod,
		NormalizeOrientation: true,
		EncodeOptions:        EncodeOptions[outputType],
	}

	// resize and transcode image
	outputImg, err = ops.Transform(decoder, opts, outputImg)
	if err != nil {
		msg := fmt.Sprintf("error transforming image, %s\n", err)
		log.Print(msg)

		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(outputImg)))

	if _, err := w.Write(outputImg); err != nil {
		log.Println("unable to output image.")
	}
	fmt.Printf("image output: %d x %d\n", outputWidth, outputHeight)

}

func main() {

	flag.StringVar(&addr, "addr", "localhost:8080", "TCP address to listen on")
	flag.StringVar(&baseURL, "baseURL", "", "default base URL for relative remote URLs")
	flag.Parse()

	http.HandleFunc("/", imageHandler)
	fmt.Printf("Starting server: %s x %s\n", baseURL, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
