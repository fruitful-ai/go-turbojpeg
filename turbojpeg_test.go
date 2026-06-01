package turbojpeg

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecompress(t *testing.T) {
	box := image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 640, Y: 480},
	}
	img := image.NewRGBA(box)
	for i := range 640 {
		for j := range 480 {
			c := color.RGBA{
				R: uint8(i % 255),
				G: uint8(j % 255),
				B: uint8((i + j) % 255),
			}
			img.SetRGBA(i, j, c)
		}
	}

	var jpegBuf bytes.Buffer
	err := jpeg.Encode(&jpegBuf, img, nil)
	require.NoError(t, err)

	decompressor, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer decompressor.Close()

	result, err := decompressor.DecompressJpegToYuv(jpegBuf.Bytes(), 160, 120)
	require.NoError(t, err)

	wantSize := 160 * 120 * 3 / 2
	if length := len(result); length != wantSize {
		t.Fatalf("Wanted %d got %d", wantSize, length)
	}

	// Check that the Y plane has variaton
	yPlane := result[:160*120]
	if bytes.Count(yPlane, []byte{yPlane[0]}) == len(yPlane) {
		t.Fatal("Y-plane has no variation")
	}
}

func TestNoPanicOnClose(t *testing.T) {
	var decompressor Decompressor
	require.Nil(t, decompressor.handle)
	decompressor.Close()
}

func TestSuggestScaling_ZeroScalingOnInvalidBytes(t *testing.T) {
	width, height, err := SuggestScaling([]byte("not jpeg"), 640, 480, Manhattan)
	require.Equal(t, width, 0)
	require.Equal(t, height, 0)
	require.Error(t, err)
}

func TestSuggestScaling(t *testing.T) {
	box := image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 640, Y: 480},
	}
	img := image.NewRGBA(box)
	var jpegBuf bytes.Buffer
	err := jpeg.Encode(&jpegBuf, img, nil)
	require.NoError(t, err)

	width, height, err := SuggestScaling(jpegBuf.Bytes(), 64, 64, Manhattan)
	require.NoError(t, err)

	// Closest supported ratio should be 1/8
	require.Equal(t, width, 640/8)
	require.Equal(t, height, 480/8)
}
