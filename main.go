package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"io/ioutil"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
)

func addOverlay(img *image.RGBA, y int, label string, fontFile string, fontSizePt float64) {
	// get and parse the font
	fontBytes, err := ioutil.ReadFile(fontFile)
	if err != nil {
		log.Printf("Error reading font: %#v\n", err)
		return
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		log.Printf("Error parsing font: %#v\n", err)
		return
	}

	// font setup
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(fontSizePt)
	c.SetClip(img.Bounds())
	c.SetSrc(image.White)
	c.SetHinting(font.HintingNone)
	c.SetDst(img)

	opts := truetype.Options{}
	opts.Size = fontSizePt
	face := truetype.NewFace(f, &opts)
	var totalWidth int

	// add the text as glyphs so we can center them over the image
	for _, x := range label {
		awidth, ok := face.GlyphAdvance(rune(x))
		if ok != true {
			log.Printf("Error getting glyph width: %#v\n", err)
			return
		}
		iwidthf := int(float64(awidth) / 64)
		totalWidth += iwidthf
	}
	pt := freetype.Pt((img.Bounds().Dx()-totalWidth)/2, y)
	c.DrawString(label, pt)
}

func getImageBuffer(overlayText string, width int) (*bytes.Buffer, error) {
	f, err := os.Open("src/images/dio.png")
	if err != nil {
		log.Printf("Error opening source image: %#v\n", err)
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(bufio.NewReader(f))
	if err != nil {
		log.Printf("Error decoding source image: %#v\n", err)
		return nil, err
	}

	b := src.Bounds()
	m := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(m, m.Bounds(), src, b.Min, draw.Src)

	addOverlay(m, 345, overlayText, "src/fonts/luximr.ttf", 126)

	// scale down if needed
	if width > 0 && width < b.Dx() {
		scaleFactor := float32(width) / float32(b.Dx())
		dst := image.NewRGBA(image.Rect(0, 0, int(float32(b.Dx())*scaleFactor), int(float32(b.Dy())*scaleFactor)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), m, b, draw.Over, nil)
		m = dst
	}

	// encode as a png
	buffer := new(bytes.Buffer)
	if err := png.Encode(buffer, m); err != nil {
		log.Printf("Error encoding resulting PNG: %#v\n", err)
		return nil, err
	}

	return buffer, nil
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	width := 0
	vars := mux.Vars(r)
	days := vars["days"]
	date := vars["date"]

	// get the number of days since the date passed in
	if date != "" {
		timeFormat := "2006-01-02"
		t, _ := time.Parse(timeFormat, date)
		duration := time.Now().Sub(t)
		days = strconv.Itoa(int(math.Floor((duration.Hours() / 24) + 0.5)))
	}

	// handle width parameter
	if qs := r.URL.RawQuery; qs != "" {
		qmap, _ := url.ParseQuery(qs)
		if len(qmap["w"]) > 0 {
			width, _ = strconv.Atoi(qmap["w"][0])
		}
	}

	// get a byte buffer of our image with the overlay
	buffer, err := getImageBuffer(days, width)
	if err != nil {
		log.Printf("Error getting image: %#v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Uhoh"))
		return
	}

	// send it!
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		log.Printf("Error writing to stream: %#v\n", err)
	}
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/{days:[0-9]{1,4}}", imageHandler)
	r.HandleFunc("/{days:[0-9]{1,4}}.png", imageHandler)
	r.HandleFunc("/{date:\\d{4}\\-\\d{2}\\-\\d{2}}", imageHandler)
	r.HandleFunc("/{date:\\d{4}\\-\\d{2}\\-\\d{2}}.png", imageHandler)
	http.Handle("/", r)

	log.Printf("Listening on http://0.0.0.0:%v...\n", os.Getenv("PORT"))
	if err := http.ListenAndServe(fmt.Sprintf(":%v", os.Getenv("PORT")), handlers.CombinedLoggingHandler(os.Stdout, r)); err != nil {
		log.Printf("%#v\n", err)
	}
}
