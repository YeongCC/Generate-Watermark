package main

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.POST("/upload/calculate", resolveImageMeta)
	r.POST("/upload/addwatermark", generateWatermark)
	r.POST("/upload/thumbnail", generateThumbnail)
	r.Run()
}

func generateThumbnail(c *gin.Context) {
	var client ClientImageSize
	c.BindJSON(&client)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	rc, err := clientDetail.cl.Bucket(client.BucketName).Object(client.Path).NewReader(ctx)

	if err != nil {
		fmt.Println("io.Copy: %v", err)
	}
	defer rc.Close()

	Image400x300 := fileNameWithoutExtSliceNotation(client.Path) + "-400x300" + fileExtSliceNotation(client.Path)

	img, err := jpeg.Decode(rc)
	if err != nil {
		panic(err)
	}
	var newFilename string
	var newWidth int
	var newHeight int
	width := client.Width
	height := img.Bounds().Dy()
	if width >= 800 {
		newFilename = client.Path
		newWidth = width
		newHeight = height
	} else if width <= 400 {
		newFilename = Image400x300
		newimage := imaging.Resize(img, 400, 300, imaging.Box)
		imgw, _ := os.Create("ImageSize.jpg")
		jpeg.Encode(imgw, newimage, &jpeg.Options{jpeg.DefaultQuality})
		newWidth = 400
		newHeight = 300
		uploadimg, err := os.Open("ImageSize.jpg")
		if err != nil {
			log.Fatalf("failed to open: %s", err)
		}

		wc := clientDetail.cl.Bucket(client.BucketName).Object(Image400x300).NewWriter(ctx)
		if _, err := io.Copy(wc, uploadimg); err != nil {
			fmt.Errorf("io.Copy: %v", err)
		}
		if err := wc.Close(); err != nil {
			fmt.Errorf("Writer.Close: %v", err)
		}
	}

	c.JSON(200, gin.H{
		"path":   newFilename,
		"width":  newWidth,
		"height": newHeight,
	})
}

func generateWatermark(c *gin.Context) {
	var client Client
	c.BindJSON(&client)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	rc, err := clientDetail.cl.Bucket(client.BucketName).Object(client.Path).NewReader(ctx)

	if err != nil {
		fmt.Println("io.Copy: %v", err)
	}
	defer rc.Close()

	ImageWatermark := fileNameWithoutExtSliceNotation(client.Path) + "-watermark" + fileExtSliceNotation(client.Path)

	first, err := jpeg.Decode(rc)
	if err != nil {
		panic("failed to decode")
	}
	watermark, err := os.Open("watermark-white.png")
	if err != nil {
		panic(err)
	}
	defer watermark.Close()
	imgwatermark, err := png.Decode(watermark)
	if err != nil {
		panic(err)
	}

	b := first.Bounds()
	watermarkbg := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))

	m := image.NewNRGBA(b)

	draw.Draw(m, b, first, image.ZP, draw.Src)
	calculateSize := (first.Bounds().Dx() / 10)

	x, y := 0, 0
	maxY := watermarkbg.Bounds().Max.Y

	finalmark := imaging.Fit(imgwatermark, calculateSize, calculateSize, imaging.Lanczos)

	for y <= maxY {
		for col := 0; col < 10; col++ {
			offset := image.Pt(x, y)
			draw.Draw(watermarkbg, finalmark.Bounds().Add(offset), finalmark, image.ZP, draw.Over)
			x += calculateSize
		}
		y += calculateSize
		x = 0
	}

	watermarkbg2 := image.Image(watermarkbg)
	mask := image.NewUniform(color.Alpha{30})
	draw.DrawMask(m, watermarkbg2.Bounds(), watermarkbg2, image.ZP, mask, image.ZP, draw.Over)

	imgw, _ := os.Create("ImageWatermark.jpg")
	jpeg.Encode(imgw, m, &jpeg.Options{jpeg.DefaultQuality})

	uploadimg, err := os.Open("ImageWatermark.jpg")
	if err != nil {
		log.Fatalf("failed to open: %s", err)
	}

	wc := clientDetail.cl.Bucket(client.BucketName).Object(ImageWatermark).NewWriter(ctx)
	if _, err := io.Copy(wc, uploadimg); err != nil {
		fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		fmt.Errorf("Writer.Close: %v", err)
	}

	c.JSON(200, gin.H{
		"path": ImageWatermark,
	})
}

func resolveImageMeta(c *gin.Context) {
	var client Client
	c.BindJSON(&client)
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	rc, err := clientDetail.cl.Bucket(client.BucketName).Object(client.Path).NewReader(ctx)

	if err != nil {
		fmt.Println("io.Copy: %v", err)
	}
	defer rc.Close()
	slurp, err := ioutil.ReadAll(rc)
	if err != nil {
		fmt.Println("Reader.Close: %v", err)
	}

	im, _, err := image.DecodeConfig(bytes.NewReader(slurp))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", bytes.NewReader(slurp), err)
	}

	width := im.Width
	height := im.Height
	fs := float64(len(slurp))
	size := HumanFileSize(fs)
	c.JSON(200, gin.H{
		"width":    width,
		"height":   height,
		"fileSize": size,
	})
}

func HumanFileSize(size float64) string {
	suffixes[0] = "B"
	suffixes[1] = "KB"
	suffixes[2] = "MB"
	suffixes[3] = "GB"
	suffixes[4] = "TB"

	base := math.Log(size) / math.Log(1024)
	getSize := Round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	getSuffix := suffixes[int(math.Floor(base))]
	return strconv.FormatFloat(getSize, 'f', -1, 64) + " " + string(getSuffix)
}

var (
	suffixes [5]string
)

func Round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func fileNameWithoutExtSliceNotation(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}

func fileExtSliceNotation(fileName string) string {
	return filepath.Ext(fileName)
}

type Client struct {
	cl         *storage.Client
	ProjectID  string `json:"projectID"`
	BucketName string `json:"bucketName"`
	Path       string `json:"path"`
}

type ClientImageSize struct {
	cl         *storage.Client
	ProjectID  string `json:"projectID"`
	BucketName string `json:"bucketName"`
	Path       string `json:"path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

var clientDetail *Client

func init() {
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "goimage-7fa42710ff97.json") // FILL IN WITH YOUR FILE PATH
	client, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	clientDetail = &Client{
		cl: client,
	}

}
