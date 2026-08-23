package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
)

const (
	ecommerceProviderReferenceMaxEdge  = 3072
	ecommerceProviderReferenceMaxBytes = 8 * 1024 * 1024
)

// compactEcommerceProviderImages changes only the in-memory copy sent to the
// provider. The original resource remains available for inspection, download,
// and later high-fidelity operations.
func compactEcommerceProviderImages(input *canvasGenerationInput) error {
	for index := range input.ReferenceImages {
		if err := compactEcommerceProviderImage(&input.ReferenceImages[index]); err != nil {
			return err
		}
	}
	if input.Mask != nil {
		return compactEcommerceProviderImage(input.Mask)
	}
	return nil
}

func compactEcommerceProviderImage(media *providerMedia) error {
	if media == nil {
		return nil
	}
	data, mimeType, err := decodeProviderImageDataURL(media.DataURL)
	if err != nil {
		return fmt.Errorf("读取电商参考图失败：%w", err)
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("解析电商参考图失败：%w", err)
	}
	bounds := decoded.Bounds()
	longEdge := providerMaxInt(bounds.Dx(), bounds.Dy())
	if len(data) <= ecommerceProviderReferenceMaxBytes && longEdge <= ecommerceProviderReferenceMaxEdge {
		media.Bytes = int64(len(data))
		media.Width = bounds.Dx()
		media.Height = bounds.Dy()
		media.MimeType = mimeType
		return nil
	}

	preserveAlpha := strings.EqualFold(format, "png") && imageHasVisibleAlpha(decoded)
	edge := providerMinInt(longEdge, ecommerceProviderReferenceMaxEdge)
	for attempt := 0; attempt < 7; attempt++ {
		candidate := decoded
		if longEdge > edge {
			width, height := scaledImageSize(bounds.Dx(), bounds.Dy(), edge)
			candidate = resizeProviderImage(decoded, width, height)
		}

		encoded, outputMime, encodeErr := encodeProviderReferenceImage(candidate, preserveAlpha)
		if encodeErr != nil {
			return fmt.Errorf("压缩电商参考图失败：%w", encodeErr)
		}
		if len(encoded) <= ecommerceProviderReferenceMaxBytes {
			media.DataURL = providerImageDataURL(outputMime, encoded)
			media.MimeType = outputMime
			media.Type = outputMime
			media.Bytes = int64(len(encoded))
			media.Width = candidate.Bounds().Dx()
			media.Height = candidate.Bounds().Dy()
			return nil
		}
		edge = providerMaxInt(1024, int(float64(edge)*0.8))
	}

	return fmt.Errorf("电商参考图压缩后仍超过 %dMB", ecommerceProviderReferenceMaxBytes/(1024*1024))
}

func decodeProviderImageDataURL(value string) ([]byte, string, error) {
	header, encoded, ok := strings.Cut(strings.TrimSpace(value), ",")
	if !ok || !strings.HasPrefix(header, "data:") || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return nil, "", errors.New("参考图不是有效的 base64 data URL")
	}
	mimeType := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errors.New("参考图内容为空")
	}
	return data, normalizedMediaMimeType(mimeType, data), nil
}

func providerImageDataURL(mimeType string, data []byte) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func encodeProviderReferenceImage(src image.Image, preserveAlpha bool) ([]byte, string, error) {
	var buffer bytes.Buffer
	if preserveAlpha {
		if err := png.Encode(&buffer, src); err != nil {
			return nil, "", err
		}
		return buffer.Bytes(), "image/png", nil
	}
	if err := jpeg.Encode(&buffer, src, &jpeg.Options{Quality: 86}); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), "image/jpeg", nil
}

func imageHasVisibleAlpha(src image.Image) bool {
	bounds := src.Bounds()
	for y := 0; y < 8; y++ {
		py := bounds.Min.Y + (y*(bounds.Dy()-1))/7
		for x := 0; x < 8; x++ {
			px := bounds.Min.X + (x*(bounds.Dx()-1))/7
			_, _, _, alpha := src.At(px, py).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}
	return false
}

func scaledImageSize(width int, height int, longEdge int) (int, int) {
	if width <= 0 || height <= 0 || longEdge <= 0 {
		return width, height
	}
	scale := float64(longEdge) / float64(providerMaxInt(width, height))
	return providerMaxInt(1, int(math.Round(float64(width)*scale))), providerMaxInt(1, int(math.Round(float64(height)*scale)))
}

func resizeProviderImage(src image.Image, width int, height int) image.Image {
	srcBounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	if width == 1 || height == 1 {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				dst.Set(x, y, src.At(srcBounds.Min.X, srcBounds.Min.Y))
			}
		}
		return dst
	}

	for y := 0; y < height; y++ {
		sy := (float64(y)+0.5)*float64(srcBounds.Dy())/float64(height) - 0.5
		y0 := int(math.Floor(sy))
		wy := sy - float64(y0)
		if y0 < 0 {
			y0, wy = 0, 0
		}
		y1 := providerMinInt(srcBounds.Dy()-1, y0+1)
		for x := 0; x < width; x++ {
			sx := (float64(x)+0.5)*float64(srcBounds.Dx())/float64(width) - 0.5
			x0 := int(math.Floor(sx))
			wx := sx - float64(x0)
			if x0 < 0 {
				x0, wx = 0, 0
			}
			x1 := providerMinInt(srcBounds.Dx()-1, x0+1)
			c00 := src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y0)
			c10 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y0)
			c01 := src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y1)
			c11 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y1)
			dst.SetRGBA(x, y, bilinearColor(c00, c10, c01, c11, wx, wy))
		}
	}
	return dst
}

func bilinearColor(c00, c10, c01, c11 color.Color, wx float64, wy float64) color.RGBA {
	interpolate := func(a00, a10, a01, a11 uint32) uint8 {
		top := float64(a00)*(1-wx) + float64(a10)*wx
		bottom := float64(a01)*(1-wx) + float64(a11)*wx
		return uint8(math.Round((top*(1-wy) + bottom*wy) / 257))
	}
	r00, g00, b00, a00 := c00.RGBA()
	r10, g10, b10, a10 := c10.RGBA()
	r01, g01, b01, a01 := c01.RGBA()
	r11, g11, b11, a11 := c11.RGBA()
	return color.RGBA{
		R: interpolate(r00, r10, r01, r11), G: interpolate(g00, g10, g01, g11),
		B: interpolate(b00, b10, b01, b11), A: interpolate(a00, a10, a01, a11),
	}
}

func providerMinInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func providerMaxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
