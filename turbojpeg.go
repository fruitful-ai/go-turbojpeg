package turbojpeg

/*
#cgo LDFLAGS: -lturbojpeg
#include <turbojpeg.h>
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"fmt"
	"image/jpeg"
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
	buf := make([]byte, dstWidth*dstHeight*3/2)
	returnCode := C.tjDecompressToYUV2(
		d.handle,
		(*C.uchar)(unsafe.Pointer(&jpegData[0])),
		C.ulong(len(jpegData)),
		(*C.uchar)(unsafe.Pointer(&buf[0])),
		C.int(dstWidth),
		1,
		C.int(dstHeight),
		C.TJFLAG_FASTDCT,
	)

	if returnCode != 0 {
		errStr := C.GoString(C.tjGetErrorStr())
		return nil, fmt.Errorf("could not decompress jpeg to yuv: %s", errStr)
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

// SuggestScaling reads the jpeg data and finds the closest possible scaling that is
// supported by turbojpeg
func SuggestScaling(jpegData []byte, wantW, wantH int) (actualW, actualH int, err error) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil {
		return 0, 0, err
	}

	type ratio struct{ num, denom int }
	ratios := []ratio{
		{1, 1}, {3, 4}, {2, 3}, {1, 2}, {3, 8}, {1, 4}, {1, 8},
	}

	bestDist := math.MaxInt32
	for _, r := range ratios {
		w := cfg.Width * r.num / r.denom
		h := cfg.Height * r.num / r.denom
		dist := abs(w-wantW) + abs(h-wantH)
		if dist < bestDist {
			bestDist = dist
			actualW, actualH = w, h
		} else if dist == bestDist && w*h > actualW*actualH {
			actualW, actualH = w, h
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
