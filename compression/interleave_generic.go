//go:build !amd64 && !arm64

package compression

// deinterleaveASM falls back to pure Go on unsupported platforms.
func deinterleaveASM(dst, src []byte) {
	n := len(src)
	if n == 0 {
		return
	}

	half := (n + 1) / 2
	for i := 0; i < half; i++ {
		dst[i*2] = src[i]
		if half+i < n {
			dst[i*2+1] = src[half+i]
		}
	}
}

// interleaveASM falls back to pure Go on unsupported platforms.
func interleaveASM(dst, src []byte) {
	n := len(src)
	if n == 0 {
		return
	}

	half := (n + 1) / 2
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			dst[i/2] = src[i]
		} else {
			dst[half+i/2] = src[i]
		}
	}
}
