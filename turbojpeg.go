package turbojpeg

/*
#cgo LDFLAGS: -lturbojpeg
#include <turbojpeg.h>
#include <stdlib.h>

// Wraps the TJSCALED macro so it's callable from Go
static int go_tjscaled(int dim, int num, int denom) {
    tjscalingfactor f = {num, denom};
    return TJSCALED(dim, f);
}
*/
import "C"
import (
	"fmt"
	"math"
	"unsafe"
)

type Decompressor struct {
	handle C.tjhandle
}

func New() (*Decompressor, error) {
	h := C.tjInitDecompress()
	if h != nil {
		return &Decompressor{handle: h}, nil
	}
	return nil, ErrCouldNotInitializeHandle
}

func (d *Decompressor) DecompressJpegToYuv(jpegData []byte, dstWidth, dstHeight int) ([]byte, error) {
	align := 16
	var imgWidth, imgHeight, subsamp, colorspace C.int
	returnCode := C.tjDecompressHeader3(
		d.handle,
		(*C.uchar)(unsafe.Pointer(&jpegData[0])),
		C.ulong(len(jpegData)),
		&imgWidth,
		&imgHeight,
		&subsamp,
		&colorspace,
	)
	if returnCode != 0 {
		errStr := C.GoString(C.tjGetErrorStr())
		return nil, fmt.Errorf("could not read jpeg header: %s", errStr)
	}

	bufSize := C.tjBufSizeYUV2(C.int(dstWidth), C.int(align), C.int(dstHeight), subsamp)
	if bufSize == 0 {
		errStr := C.GoString(C.tjGetErrorStr())
		return nil, fmt.Errorf("could not compute yuv buffer size: %s", errStr)
	}

	buf := make([]byte, bufSize)
	returnCode = C.tjDecompressToYUV2(
		d.handle,
		(*C.uchar)(unsafe.Pointer(&jpegData[0])),
		C.ulong(len(jpegData)),
		(*C.uchar)(unsafe.Pointer(&buf[0])),
		C.int(dstWidth),
		C.int(align),
		C.int(dstHeight),
		C.TJFLAG_FASTDCT,
	)

	if returnCode != 0 {
		errStr := C.GoString(C.tjGetErrorStr())
		return nil, fmt.Errorf("could not decompress jpeg to yuv: %s", errStr)
	}
	return buf, nil
}

func (d *Decompressor) DecompressJpegToRGB(jpegData []byte, dstWidth, dstHeight int) ([]byte, error) {
	pitch := dstWidth * 4
	buf := make([]byte, pitch*dstHeight)
	returnCode := C.tjDecompress2(
		d.handle,
		(*C.uchar)(unsafe.Pointer(&jpegData[0])),
		C.ulong(len(jpegData)),
		(*C.uchar)(unsafe.Pointer(&buf[0])),
		C.int(dstWidth),
		C.int(pitch),
		C.int(dstHeight),
		C.TJPF_RGBA,
		C.TJFLAG_FASTDCT,
	)
	if returnCode != 0 {
		errStr := C.GoString(C.tjGetErrorStr())
		return nil, fmt.Errorf("could not decompress jpeg to rgb: %s", errStr)
	}
	return buf, nil
}

func (d *Decompressor) Close() {
	if d.handle == nil {
		return
	}
	C.tjDestroy(d.handle)
	d.handle = nil
}

type DistanceFuncInt func(x0, x1, y0, y1 int) int

// SuggestScaling reads the jpeg data and finds the closest possible scaling that is
// supported by turbojpeg
func SuggestScaling(jpegData []byte, wantW, wantH int, dist DistanceFuncInt) (actualW, actualH int, err error) {
	var imgWidth, imgHeight, subsamp, colorspace C.int

	d, err := New()
	if err != nil {
		return 0, 0, err
	}
	defer d.Close()

	returnCode := C.tjDecompressHeader3(
		d.handle,
		(*C.uchar)(unsafe.Pointer(&jpegData[0])),
		C.ulong(len(jpegData)),
		&imgWidth,
		&imgHeight,
		&subsamp,
		&colorspace,
	)
	if returnCode != 0 {
		return 0, 0, fmt.Errorf("could not read jpeg header: %s", C.GoString(C.tjGetErrorStr()))
	}

	var numFactors C.int
	factors := C.tjGetScalingFactors(&numFactors)
	if factors == nil {
		return 0, 0, fmt.Errorf("could not get scaling factors")
	}
	factorSlice := unsafe.Slice(factors, int(numFactors))

	bestDist := math.MaxInt32
	for i := 0; i < int(numFactors); i++ {
		f := factorSlice[i]
		num, denom := int(f.num), int(f.denom)
		if num > denom {
			continue // upscaling only
		}

		w := int(C.go_tjscaled(C.int(imgWidth), C.int(num), C.int(denom)))
		h := int(C.go_tjscaled(C.int(imgHeight), C.int(num), C.int(denom)))
		we, he := w & ^1, h & ^1

		d := dist(wantW, we, wantH, he)
		if d < bestDist {
			bestDist = d
			actualW, actualH = we, he
		} else if d == bestDist && we*he > actualW*actualH {
			actualW, actualH = we, he
		}
	}
	return actualW, actualH, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func Manhattan(x0, x1, y0, y1 int) int {
	return abs(x1-x0) + abs(y1-y0)
}
