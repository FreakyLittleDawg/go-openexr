// Package predictor implements the horizontal differencing predictor
// used by OpenEXR compression algorithms.
//
// The predictor converts absolute pixel values to differences from
// the previous value, which tends to produce more compressible data
// for images with local coherence.
package predictor

// Encode applies horizontal differencing to the data in place.
// The first byte remains unchanged, subsequent bytes become
// differences from their predecessor.
//
// This is used before compression to improve compression ratios.
func Encode(data []byte) {
	n := len(data)
	if n < 2 {
		return
	}

	// Work backwards to preserve values we need
	// Process in chunks of 8 for better pipelining
	i := n - 1
	for ; i >= 8; i -= 8 {
		data[i] = data[i] - data[i-1]
		data[i-1] = data[i-1] - data[i-2]
		data[i-2] = data[i-2] - data[i-3]
		data[i-3] = data[i-3] - data[i-4]
		data[i-4] = data[i-4] - data[i-5]
		data[i-5] = data[i-5] - data[i-6]
		data[i-6] = data[i-6] - data[i-7]
		data[i-7] = data[i-7] - data[i-8]
	}

	// Handle remaining bytes
	for ; i >= 1; i-- {
		data[i] = data[i] - data[i-1]
	}
}

// Decode reverses horizontal differencing in place.
// Each byte becomes the sum of itself and all previous bytes.
//
// This is used after decompression to restore the original values.
func Decode(data []byte) {
	n := len(data)
	if n < 2 {
		return
	}

	// Process in chunks of 8 for better pipelining
	i := 1
	for ; i+7 < n; i += 8 {
		data[i] = data[i] + data[i-1]
		data[i+1] = data[i+1] + data[i]
		data[i+2] = data[i+2] + data[i+1]
		data[i+3] = data[i+3] + data[i+2]
		data[i+4] = data[i+4] + data[i+3]
		data[i+5] = data[i+5] + data[i+4]
		data[i+6] = data[i+6] + data[i+5]
		data[i+7] = data[i+7] + data[i+6]
	}

	// Handle remaining bytes
	for ; i < n; i++ {
		data[i] = data[i] + data[i-1]
	}
}

// EncodeRow applies horizontal differencing to a single scanline,
// treating it as interleaved channel data.
//
// For OpenEXR, the predictor operates on individual bytes within
// each channel's data, not across channels.
func EncodeRow(data []byte, width, numChannels, bytesPerPixel int) {
	if width == 0 || numChannels == 0 || bytesPerPixel == 0 {
		return
	}

	// OpenEXR applies predictor to the interleaved byte stream
	// Each byte is predicted from the previous byte
	Encode(data)
}

// DecodeRow reverses horizontal differencing for a scanline.
func DecodeRow(data []byte, width, numChannels, bytesPerPixel int) {
	if width == 0 || numChannels == 0 || bytesPerPixel == 0 {
		return
	}

	Decode(data)
}
