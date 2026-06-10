package turbojpeg

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
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

func TestSuggestScaling_MustMatchTjscaled(t *testing.T) {
	// Dimensions where Go truncation differs from TJSCALED ceiling.
	// The old code used go truncation (w * num / denom) which
	// can differ from TJSCALED by 1+ pixel, causing the buffer
	// layout to mismatch what tjDecompressToYUV2 actually produces.
	w, h := 2341, 63
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{
				uint8(x % 255), uint8(y % 255), uint8((x + y) % 255), 255,
			})
		}
	}
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	require.NoError(t, err)

	// For 2341×63 with target 640×480:
	//   3/8 = 877 → TJSCALED = 878 → still the closest supported
	//   1/4 = 585 → TJSCALED = 586
	// Manhattan distance picks 1/4 (586×16 is closer than 878×24)
	suggestedW, suggestedH, err := SuggestScaling(buf.Bytes(), 640, 480, Manhattan)
	require.NoError(t, err)

	// Must use TJSCALED ceiling matching libjpeg-turbo
	require.Equal(t, 586, suggestedW, "must use TJSCALED ceiling, not truncation")
	require.Equal(t, 16, suggestedH, "must use TJSCALED ceiling, not truncation")

	// Ceil-to-even ensures 4:2:0 chroma planes are sized correctly
	require.True(t, suggestedW%2 == 0, "width must be even for 4:2:0")
	require.True(t, suggestedH%2 == 0, "height must be even for 4:2:0")

	// Verify decompression produces correctly-sized YUV
	d, err := New()
	require.NoError(t, err)
	defer d.Close()

	yuv, err := d.DecompressJpegToYuv(buf.Bytes(), suggestedW, suggestedH)
	require.NoError(t, err)
	require.Equal(t, suggestedW*suggestedH*3/2, len(yuv))
}

func TestDecompressYuv_Grayscale(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 64, 64))
	for x := range 64 {
		for y := range 64 {
			gray.SetGray(x, y, color.Gray{Y: uint8(x + y)})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, gray, nil)
	require.NoError(t, err)

	d, err := New()
	require.NoError(t, err)
	defer d.Close()

	result, err := d.DecompressJpegToYuv(buf.Bytes(), 64, 64)
	require.NoError(t, err)

	// Grayscale: only Y plane is valid (no chroma)
	require.Equal(t, 64*64, len(result), "grayscale YUV should be just Y plane")

	yPlane := result[:64*64]
	require.NotEqual(t, bytes.Count(yPlane, []byte{yPlane[0]}), len(yPlane),
		"Y-plane should have variation")
}

func TestDecompressYuv_OddDimensions(t *testing.T) {
	w, h := 161, 121
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 255),
				G: uint8(y % 255),
				B: uint8((x + y) % 255),
			})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	require.NoError(t, err)

	d, err := New()
	require.NoError(t, err)
	defer d.Close()

	result, err := d.DecompressJpegToYuv(buf.Bytes(), w, h)
	require.NoError(t, err)

	// Should not panic or error on odd dimensions
	// Y plane should fill w*h bytes
	yPlane := result[:w*h]
	require.NotEqual(t, bytes.Count(yPlane, []byte{yPlane[0]}), len(yPlane),
		"Y-plane should have variation")
}

func TestDecompressRGB(t *testing.T) {
	w, h := 64, 64
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 4),
				G: uint8(y * 4),
				B: uint8((x + y) * 2),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	require.NoError(t, err)

	d, err := New()
	require.NoError(t, err)
	defer d.Close()

	result, err := d.DecompressJpegToRGB(buf.Bytes(), w, h)
	require.NoError(t, err)

	require.Equal(t, w*h*4, len(result))

	// Alpha should always be 255 (TJPF_RGBA fills it)
	for i := 3; i < len(result); i += 4 {
		require.Equal(t, uint8(255), result[i], "alpha at pixel %d", i/4)
	}
}

func TestNoDirstortionOnDecompress(t *testing.T) {
	w, h := 2401, 153
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h / 2 {
			img.Set(x, y, color.RGBA{G: 255, A: 255})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	require.NoError(t, err)

	jpegBytes := buf.Bytes()
	width, height, err := SuggestScaling(jpegBytes, 640, 240, Manhattan)
	require.NoError(t, err)
	require.Equal(t, 602, width)
	require.Equal(t, 40, height)

	decompressor, err := New()
	require.NoError(t, err)

	yuvBytes, err := decompressor.DecompressJpegToYuv(jpegBytes, width, height)

	saveImage := false
	if saveImage {
		f, err := os.Create("decompressed.yuv")
		defer f.Close()
		require.NoError(t, err)
		_, err = f.Write(yuvBytes)
		require.NoError(t, err)

		fOrig, err := os.Create("orig.jpg")
		require.NoError(t, err)
		defer fOrig.Close()
		_, err = fOrig.Write(jpegBytes)
		require.NoError(t, err)
	}
}
