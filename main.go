package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.POST("/upload/calculate", func(c *gin.Context) {
		f, err := c.FormFile("file_input")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		blobFile, err := f.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		width, height := resolveImageMeta(blobFile, f.Filename)
		fs := float64(f.Size)
		c.JSON(200, gin.H{
			"width":    width,
			"height":   height,
			"fileSize": HumanFileSize(fs),
		})
	})
	r.POST("/upload/addwatermark", func(c *gin.Context) {
		f, err := c.FormFile("file_input")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		blobFile, err := f.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		image := generateWatermark(blobFile, f.Filename)

		c.JSON(200, gin.H{
			"path": "image/" + image,
		})
	})
	r.POST("/upload/thumbnail", func(c *gin.Context) {
		f, err := c.FormFile("file_input")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		blobFile, err := f.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		width, height, image := generateThumbnail(blobFile, f.Filename)

		c.JSON(200, gin.H{
			"path":   "image/" + image,
			"width":  width,
			"height": height,
		})
	})
	r.Run()
}

func generateThumbnail(file multipart.File, filename string) (int, int, string) {

	img, err := jpeg.Decode(file)
	if err != nil {
		panic(err)
	}
	var newFilename string
	var newWidth int
	var newHeight int
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	if width >= 800 {
		newFilename = filename
		fmt.Print(width)
		fmt.Print(height)
		newWidth = width
		newHeight = height
	} else if width <= 400 {
		newFilename = (fileNameWithoutExtSliceNotation(filename) + "-400x300" + fileExtSliceNotation(filename))
		newimage := imaging.Resize(img, 400, 300, imaging.Box)
		imaging.Save(newimage, newFilename)
		newWidth = 400
		newHeight = 300
	}

	return newWidth, newHeight, newFilename
}

func generateWatermark(file multipart.File, filename string) string {

	first, err := jpeg.Decode(file)
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
	imgw, _ := os.Create(fileNameWithoutExtSliceNotation(filename) + "-watermark" + fileExtSliceNotation(filename))
	jpeg.Encode(imgw, m, &jpeg.Options{jpeg.DefaultQuality})
	return (fileNameWithoutExtSliceNotation(filename) + "-watermark" + fileExtSliceNotation(filename))
}

func fileNameWithoutExtSliceNotation(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}

func fileExtSliceNotation(fileName string) string {
	return filepath.Ext(fileName)
}

func resolveImageMeta(file multipart.File, object string) (int, int) {

	im, _, err := image.DecodeConfig(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", file, err)
	}
	fmt.Printf("%s %d %d\n", file, im.Width, im.Height)

	return im.Width, im.Height
}

func HumanFileSize(size float64) string {
	fmt.Println(size)
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
