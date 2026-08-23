package service

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

func TestCompactEcommerceProviderImageBoundsOutboundCopy(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4000, 5000))
	for y := 0; y < 5000; y += 17 {
		for x := 0; x < 4000; x += 17 {
			source.SetRGBA(x, y, colorForTest(x, y))
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	original := encoded.Bytes()
	media := providerMedia{MimeType: "image/jpeg", Type: "image/jpeg", DataURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(original)}
	if err := compactEcommerceProviderImage(&media); err != nil {
		t.Fatalf("compactEcommerceProviderImage() error = %v", err)
	}
	data, _, err := decodeProviderImageDataURL(media.DataURL)
	if err != nil {
		t.Fatalf("decodeProviderImageDataURL() error = %v", err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("image.DecodeConfig() error = %v", err)
	}
	if providerMaxInt(config.Width, config.Height) > ecommerceProviderReferenceMaxEdge {
		t.Fatalf("outbound long edge = %d, want <= %d", providerMaxInt(config.Width, config.Height), ecommerceProviderReferenceMaxEdge)
	}
	if len(data) > ecommerceProviderReferenceMaxBytes {
		t.Fatalf("outbound bytes = %d, want <= %d", len(data), ecommerceProviderReferenceMaxBytes)
	}
	if media.Bytes != int64(len(data)) || media.Width != config.Width || media.Height != config.Height {
		t.Fatalf("media metadata not updated: bytes=%d dimensions=%dx%d decoded=%dx%d", media.Bytes, media.Width, media.Height, config.Width, config.Height)
	}
	if strings.Contains(media.DataURL, base64.StdEncoding.EncodeToString(original)) {
		t.Fatal("outbound copy still contains the original data URL")
	}

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	if err := writeMediaPart(writer, "image", media); err != nil {
		t.Fatalf("writeMediaPart() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart.Writer.Close() error = %v", err)
	}
	reader := multipart.NewReader(bytes.NewReader(multipartBody.Bytes()), writer.Boundary())
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("multipart.Reader.NextPart() error = %v", err)
	}
	uploaded, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("io.ReadAll(multipart image) error = %v", err)
	}
	if !bytes.Equal(uploaded, data) {
		t.Fatalf("multipart image bytes = %d, want compacted bytes = %d", len(uploaded), len(data))
	}
	if bytes.Equal(uploaded, original) {
		t.Fatal("multipart upload still contains the original reference image")
	}
}

func TestAnyInt64HandlesJSONNumberFormatting(t *testing.T) {
	if got := anyInt64(float64(34655811)); got != 34655811 {
		t.Fatalf("anyInt64(float64) = %d, want 34655811", got)
	}
	if got := anyInt64("3.4655811e+07"); got != 34655811 {
		t.Fatalf("anyInt64(scientific notation) = %d, want 34655811", got)
	}
}

func colorForTest(x int, y int) color.RGBA {
	return color.RGBA{R: uint8((x / 17) % 255), G: uint8((y / 17) % 255), B: uint8((x + y) % 255), A: 0xff}
}
