package exr

import (
	"testing"

	"github.com/mrjoshuak/go-openexr/internal/xdr"
)

func TestPixelType(t *testing.T) {
	tests := []struct {
		pt   PixelType
		str  string
		size int
	}{
		{PixelTypeUint, "uint", 4},
		{PixelTypeHalf, "half", 2},
		{PixelTypeFloat, "float", 4},
		{PixelType(99), "unknown", 0},
	}

	for _, tt := range tests {
		if s := tt.pt.String(); s != tt.str {
			t.Errorf("%v.String() = %q, want %q", tt.pt, s, tt.str)
		}
		if sz := tt.pt.Size(); sz != tt.size {
			t.Errorf("%v.Size() = %d, want %d", tt.pt, sz, tt.size)
		}
	}
}

func TestNewChannel(t *testing.T) {
	c := NewChannel("R", PixelTypeHalf)
	if c.Name != "R" {
		t.Errorf("Name = %q, want %q", c.Name, "R")
	}
	if c.Type != PixelTypeHalf {
		t.Errorf("Type = %v, want %v", c.Type, PixelTypeHalf)
	}
	if c.XSampling != 1 || c.YSampling != 1 {
		t.Errorf("Sampling = %dx%d, want 1x1", c.XSampling, c.YSampling)
	}
	if c.PLinear {
		t.Error("PLinear should be false by default")
	}
}

func TestChannelLayer(t *testing.T) {
	tests := []struct {
		name     string
		layer    string
		baseName string
	}{
		{"R", "", "R"},
		{"diffuse.R", "diffuse", "R"},
		{"light.specular.R", "light.specular", "R"},
		{"A", "", "A"},
	}

	for _, tt := range tests {
		c := Channel{Name: tt.name}
		if l := c.Layer(); l != tt.layer {
			t.Errorf("Channel(%q).Layer() = %q, want %q", tt.name, l, tt.layer)
		}
		if bn := c.BaseName(); bn != tt.baseName {
			t.Errorf("Channel(%q).BaseName() = %q, want %q", tt.name, bn, tt.baseName)
		}
	}
}

func TestChannelList(t *testing.T) {
	cl := NewChannelList()
	if cl.Len() != 0 {
		t.Errorf("Len() = %d, want 0", cl.Len())
	}

	// Add channels
	if !cl.Add(NewChannel("R", PixelTypeHalf)) {
		t.Error("Add(R) should return true")
	}
	if !cl.Add(NewChannel("G", PixelTypeHalf)) {
		t.Error("Add(G) should return true")
	}
	if !cl.Add(NewChannel("B", PixelTypeHalf)) {
		t.Error("Add(B) should return true")
	}

	if cl.Len() != 3 {
		t.Errorf("Len() = %d, want 3", cl.Len())
	}

	// Duplicate should fail
	if cl.Add(NewChannel("R", PixelTypeFloat)) {
		t.Error("Adding duplicate R should return false")
	}

	// Get channel
	r := cl.Get("R")
	if r == nil {
		t.Fatal("Get(R) returned nil")
	}
	if r.Type != PixelTypeHalf {
		t.Errorf("Get(R).Type = %v, want Half", r.Type)
	}

	// Get non-existent
	if cl.Get("X") != nil {
		t.Error("Get(X) should return nil")
	}

	// At
	c0 := cl.At(0)
	if c0.Name != "R" {
		t.Errorf("At(0).Name = %q, want %q", c0.Name, "R")
	}

	// Names
	names := cl.Names()
	if len(names) != 3 {
		t.Errorf("Names() len = %d, want 3", len(names))
	}
}

func TestChannelListRGBA(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))
	cl.Add(NewChannel("B", PixelTypeHalf))

	if !cl.HasRGB() {
		t.Error("HasRGB() should be true")
	}
	if cl.HasAlpha() {
		t.Error("HasAlpha() should be false")
	}
	if cl.HasRGBA() {
		t.Error("HasRGBA() should be false")
	}

	cl.Add(NewChannel("A", PixelTypeHalf))
	if !cl.HasRGBA() {
		t.Error("HasRGBA() should be true after adding A")
	}
}

func TestChannelListLayers(t *testing.T) {
	cl := NewChannelList()
	cl.Add(Channel{Name: "R"})
	cl.Add(Channel{Name: "G"})
	cl.Add(Channel{Name: "B"})
	cl.Add(Channel{Name: "diffuse.R"})
	cl.Add(Channel{Name: "diffuse.G"})
	cl.Add(Channel{Name: "diffuse.B"})
	cl.Add(Channel{Name: "specular.R"})

	layers := cl.Layers()
	if len(layers) != 2 {
		t.Errorf("Layers() len = %d, want 2", len(layers))
	}

	// Root layer channels
	root := cl.ChannelsInLayer("")
	if len(root) != 3 {
		t.Errorf("ChannelsInLayer('') len = %d, want 3", len(root))
	}

	// Diffuse layer channels
	diffuse := cl.ChannelsInLayer("diffuse")
	if len(diffuse) != 3 {
		t.Errorf("ChannelsInLayer('diffuse') len = %d, want 3", len(diffuse))
	}
}

func TestChannelListSorting(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("Z", PixelTypeFloat))
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("A", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))

	// Sort by name
	cl.SortByName()
	if cl.At(0).Name != "A" {
		t.Errorf("After SortByName(), At(0).Name = %q, want %q", cl.At(0).Name, "A")
	}
	if cl.At(1).Name != "G" {
		t.Errorf("After SortByName(), At(1).Name = %q, want %q", cl.At(1).Name, "G")
	}

	// Sort for compression (by type, then name)
	cl.SortForCompression()
	// Half (type 1) comes before Float (type 2)
	if cl.At(0).Type != PixelTypeHalf {
		t.Errorf("After SortForCompression(), At(0).Type = %v, want Half", cl.At(0).Type)
	}
	// Z (Float) should be last
	if cl.At(3).Name != "Z" {
		t.Errorf("After SortForCompression(), At(3).Name = %q, want %q", cl.At(3).Name, "Z")
	}
}

func TestChannelListBytes(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))  // 2 bytes
	cl.Add(NewChannel("G", PixelTypeHalf))  // 2 bytes
	cl.Add(NewChannel("B", PixelTypeHalf))  // 2 bytes
	cl.Add(NewChannel("Z", PixelTypeFloat)) // 4 bytes

	if bpp := cl.BytesPerPixel(); bpp != 10 {
		t.Errorf("BytesPerPixel() = %d, want 10", bpp)
	}

	// 100 pixel wide scanline
	if bps := cl.BytesPerScanline(100); bps != 1000 {
		t.Errorf("BytesPerScanline(100) = %d, want 1000", bps)
	}
}

func TestChannelListSubsampling(t *testing.T) {
	cl := NewChannelList()
	cl.Add(Channel{Name: "Y", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "RY", Type: PixelTypeHalf, XSampling: 2, YSampling: 2})
	cl.Add(Channel{Name: "BY", Type: PixelTypeHalf, XSampling: 2, YSampling: 2})

	// For 100 pixel width:
	// Y: 100 * 2 = 200 bytes
	// RY: 50 * 2 = 100 bytes (2x subsampled)
	// BY: 50 * 2 = 100 bytes (2x subsampled)
	// Total: 400 bytes
	if bps := cl.BytesPerScanline(100); bps != 400 {
		t.Errorf("BytesPerScanline(100) with subsampling = %d, want 400", bps)
	}
}

func TestChannelListSerialization(t *testing.T) {
	original := NewChannelList()
	original.Add(Channel{Name: "R", Type: PixelTypeHalf, XSampling: 1, YSampling: 1, PLinear: false})
	original.Add(Channel{Name: "G", Type: PixelTypeHalf, XSampling: 1, YSampling: 1, PLinear: false})
	original.Add(Channel{Name: "B", Type: PixelTypeHalf, XSampling: 1, YSampling: 1, PLinear: false})
	original.Add(Channel{Name: "Z", Type: PixelTypeFloat, XSampling: 1, YSampling: 1, PLinear: true})

	w := xdr.NewBufferWriter(256)
	WriteChannelList(w, original)

	r := xdr.NewReader(w.Bytes())
	result, err := ReadChannelList(r)
	if err != nil {
		t.Fatalf("ReadChannelList() error = %v", err)
	}

	if result.Len() != original.Len() {
		t.Errorf("Len() = %d, want %d", result.Len(), original.Len())
	}

	for i := 0; i < original.Len(); i++ {
		orig := original.At(i)
		res := result.At(i)
		if res.Name != orig.Name {
			t.Errorf("Channel[%d].Name = %q, want %q", i, res.Name, orig.Name)
		}
		if res.Type != orig.Type {
			t.Errorf("Channel[%d].Type = %v, want %v", i, res.Type, orig.Type)
		}
		if res.XSampling != orig.XSampling {
			t.Errorf("Channel[%d].XSampling = %d, want %d", i, res.XSampling, orig.XSampling)
		}
		if res.YSampling != orig.YSampling {
			t.Errorf("Channel[%d].YSampling = %d, want %d", i, res.YSampling, orig.YSampling)
		}
		if res.PLinear != orig.PLinear {
			t.Errorf("Channel[%d].PLinear = %v, want %v", i, res.PLinear, orig.PLinear)
		}
	}
}

func TestChannelListChannels(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))

	channels := cl.Channels()
	if len(channels) != 2 {
		t.Errorf("Channels() len = %d, want 2", len(channels))
	}

	// Modify returned slice shouldn't affect original
	channels[0].Name = "X"
	if cl.At(0).Name != "R" {
		t.Error("Channels() should return a copy")
	}
}

func TestChannelListReadError(t *testing.T) {
	// Test reading with insufficient data
	r := xdr.NewReader([]byte{'R', 0}) // Just the name, no type/properties
	_, err := ReadChannelList(r)
	if err == nil {
		t.Error("ReadChannelList with insufficient data should error")
	}
}

func TestChannelListReadErrorMissingPLinear(t *testing.T) {
	// Name + type but no pLinear
	data := []byte{'R', 0, 0, 0, 0, 0} // name "R", type 0 (4 bytes)
	r := xdr.NewReader(data)
	_, err := ReadChannelList(r)
	if err == nil {
		t.Error("ReadChannelList with missing pLinear should error")
	}
}

func TestChannelListReadErrorMissingSampling(t *testing.T) {
	// Name + type + pLinear + reserved but no sampling
	data := []byte{'R', 0, 0, 0, 0, 0, 0, 0, 0, 0} // name "R", type, pLinear, reserved
	r := xdr.NewReader(data)
	_, err := ReadChannelList(r)
	if err == nil {
		t.Error("ReadChannelList with missing sampling should error")
	}
}

func TestChannelListReadErrorMissingReserved(t *testing.T) {
	// Name + type + pLinear but no reserved bytes
	data := []byte{'R', 0, 0, 0, 0, 0, 0} // name "R", type, pLinear only
	r := xdr.NewReader(data)
	_, err := ReadChannelList(r)
	if err == nil {
		t.Error("ReadChannelList with missing reserved bytes should error")
	}
}

func TestChannelListReadErrorMissingYSampling(t *testing.T) {
	// Name + type + pLinear + reserved + xSampling but no ySampling
	data := []byte{
		'R', 0, // name "R"
		0, 0, 0, 0, // type (4 bytes)
		0,       // pLinear (1 byte)
		0, 0, 0, // reserved (3 bytes)
		1, 0, 0, 0, // xSampling (4 bytes, value 1)
	}
	r := xdr.NewReader(data)
	_, err := ReadChannelList(r)
	if err == nil {
		t.Error("ReadChannelList with missing ySampling should error")
	}
}
