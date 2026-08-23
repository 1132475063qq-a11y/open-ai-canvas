package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"strings"
)

const ecommerceCompositionHashAlgorithm = "dhash128-v1"

// ecommerceImageCompositionFingerprint samples only 81 pixels, so 4K result
// persistence gains duplicate detection without another full-resolution pass.
func ecommerceImageCompositionFingerprint(data []byte) (string, bool) {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", false
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 9 || bounds.Dy() < 9 {
		return "", false
	}
	var samples [9][9]uint16
	minimum, maximum := uint16(65535), uint16(0)
	for y := 0; y < 9; y++ {
		sourceY := bounds.Min.Y + ((2*y+1)*bounds.Dy())/(2*9)
		if sourceY >= bounds.Max.Y {
			sourceY = bounds.Max.Y - 1
		}
		for x := 0; x < 9; x++ {
			sourceX := bounds.Min.X + ((2*x+1)*bounds.Dx())/(2*9)
			if sourceX >= bounds.Max.X {
				sourceX = bounds.Max.X - 1
			}
			red, green, blue, _ := decoded.At(sourceX, sourceY).RGBA()
			luma := uint16((299*red + 587*green + 114*blue) / 1000)
			samples[y][x] = luma
			if luma < minimum {
				minimum = luma
			}
			if luma > maximum {
				maximum = luma
			}
		}
	}
	// Near-flat images have unstable directional hashes and are left for human QA.
	if int(maximum)-int(minimum) < 4096 {
		return "", false
	}
	var horizontal, vertical uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			horizontal <<= 1
			if samples[y][x] > samples[y][x+1] {
				horizontal |= 1
			}
			vertical <<= 1
			if samples[y][x] > samples[y+1][x] {
				vertical |= 1
			}
		}
	}
	return fmt.Sprintf("%016x%016x", horizontal, vertical), true
}

// enrichEcommerceResultCompositionFingerprint stamps the first-party hash on
// image results before data URLs are moved into resource storage. Providers
// that return a remote URL cannot be sampled here and remain UNCERTAIN until
// the human QA step; they must never be treated as distinct by assumption.
func enrichEcommerceResultCompositionFingerprint(result map[string]interface{}) map[string]interface{} {
	if result == nil {
		return result
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return result
	}
	var normalized map[string]interface{}
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return result
	}
	images, ok := normalized["images"].([]interface{})
	if !ok {
		return normalized
	}
	for index, rawImage := range images {
		imageValue, ok := rawImage.(map[string]interface{})
		if !ok || validEcommerceCompositionHash(imageValue) {
			continue
		}
		for _, key := range []string{"dataUrl", "content", "url", "coverUrl"} {
			raw, _ := imageValue[key].(string)
			data, ok := decodeEcommerceImageDataURL(raw)
			if !ok {
				continue
			}
			hash, hashOK := ecommerceImageCompositionFingerprint(data)
			if hashOK {
				imageValue["compositionHash"] = hash
				imageValue["compositionHashAlgorithm"] = ecommerceCompositionHashAlgorithm
			}
			break
		}
		images[index] = imageValue
	}
	normalized["images"] = images
	return normalized
}

func validEcommerceCompositionHash(image map[string]interface{}) bool {
	hash, hashOK := image["compositionHash"].(string)
	algorithm, algorithmOK := image["compositionHashAlgorithm"].(string)
	return algorithmOK && strings.TrimSpace(algorithm) == ecommerceCompositionHashAlgorithm && hashOK && len(strings.TrimSpace(hash)) == 32
}

func decodeEcommerceImageDataURL(raw string) ([]byte, bool) {
	header, encoded, ok := strings.Cut(strings.TrimSpace(raw), ",")
	if !ok || !strings.HasPrefix(header, "data:image/") || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	return data, err == nil && len(data) > 0
}
