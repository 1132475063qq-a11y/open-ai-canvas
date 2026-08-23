package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestEnrichEcommerceResultCompositionFingerprint(t *testing.T) {
	imageData := patternedEcommerceTestImage(t)
	result := map[string]interface{}{
		"mode":   "image",
		"images": []map[string]string{{"dataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData)}},
	}

	enriched := enrichEcommerceResultCompositionFingerprint(result)
	images, ok := enriched["images"].([]interface{})
	if !ok || len(images) != 1 {
		t.Fatalf("images = %#v", enriched["images"])
	}
	imageValue, ok := images[0].(map[string]interface{})
	if !ok {
		t.Fatalf("image value = %#v", images[0])
	}
	if imageValue["compositionHashAlgorithm"] != ecommerceCompositionHashAlgorithm {
		t.Fatalf("compositionHashAlgorithm = %#v", imageValue["compositionHashAlgorithm"])
	}
	hash, ok := imageValue["compositionHash"].(string)
	if !ok || len(hash) != 32 {
		t.Fatalf("compositionHash = %#v", imageValue["compositionHash"])
	}
}

func TestEnrichEcommerceResultCompositionFingerprintLeavesRemoteURLUnverified(t *testing.T) {
	result := map[string]interface{}{"images": []map[string]string{{"url": "https://media.example.com/generated.png"}}}
	enriched := enrichEcommerceResultCompositionFingerprint(result)
	encoded, err := json.Marshal(enriched)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("compositionHash")) {
		t.Fatalf("remote result was treated as locally fingerprinted: %s", encoded)
	}
}

func patternedEcommerceTestImage(t *testing.T) []byte {
	t.Helper()
	imageValue := image.NewGray(image.Rect(0, 0, 17, 17))
	for y := 0; y < 17; y++ {
		for x := 0; x < 17; x++ {
			imageValue.SetGray(x, y, color.Gray{Y: uint8((x*17 + y*11) % 255)})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imageValue); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}
