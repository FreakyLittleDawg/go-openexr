// Package exrid provides ID Manifest support for OpenEXR files.
//
// ID Manifests map numeric IDs (stored in image channels) to text strings
// (object names, material names, etc.). This is essential for:
//
//   - Cryptomatte: Industry-standard for object/material mattes
//   - Object ID passes: From 3D renderers
//   - Deep compositing: Selecting objects by ID
//
// Example usage:
//
//	manifest := exrid.NewManifest()
//	group := manifest.AddGroup([]string{"objectId.R", "objectId.G"}, []string{"object"})
//	group.InsertHashed("Hero")
//	group.InsertHashed("Background")
//	exrid.SetManifest(header, manifest)
package exrid

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/mrjoshuak/go-openexr/exr"
)

// ===========================================
// Types
// ===========================================

// IDLifetime indicates how long an ID-to-name mapping is valid.
type IDLifetime int

const (
	// LifetimeFrame means mapping may change every frame.
	LifetimeFrame IDLifetime = 0
	// LifetimeShot means mapping is consistent within a shot.
	LifetimeShot IDLifetime = 1
	// LifetimeStable means mapping is consistent forever.
	LifetimeStable IDLifetime = 2
)

// HashScheme identifies how IDs are generated from names.
type HashScheme string

const (
	// HashUnknown means the hash scheme is not known.
	HashUnknown HashScheme = "unknown"
	// HashNone means there is no relationship between text and ID.
	HashNone HashScheme = "none"
	// HashCustom means a custom hashing scheme is used.
	HashCustom HashScheme = "custom"
	// HashMurmur3_32 means MurmurHash3 32-bit is used (Cryptomatte standard).
	HashMurmur3_32 HashScheme = "MurmurHash3_32"
	// HashMurmur3_64 means MurmurHash3 64-bit is used.
	HashMurmur3_64 HashScheme = "MurmurHash3_64"
)

// EncodingScheme identifies how IDs are stored in channels.
type EncodingScheme string

const (
	// EncodeID means 32-bit ID in single UINT channel.
	EncodeID EncodingScheme = "id"
	// EncodeID2 means 64-bit ID in two channels.
	EncodeID2 EncodingScheme = "id2"
)

// ChannelGroupManifest describes ID mappings for a group of channels.
type ChannelGroupManifest struct {
	// Channels lists the channel names this manifest applies to.
	Channels []string
	// Components lists the component names (e.g., "object", "material").
	Components []string
	// Lifetime indicates how long mappings are valid.
	Lifetime IDLifetime
	// HashScheme identifies how IDs are generated.
	HashScheme HashScheme
	// EncodingScheme identifies how IDs are stored.
	EncodingScheme EncodingScheme
	// Entries maps ID -> component values.
	Entries map[uint64][]string
}

// Manifest contains all ID manifests for a file.
type Manifest struct {
	Groups []ChannelGroupManifest
}

// ===========================================
// Attribute names
// ===========================================

const (
	// AttrIDManifest is the standard attribute name for ID manifests.
	AttrIDManifest = "idmanifest"
	// AttrCryptomatte is the prefix for Cryptomatte metadata attributes.
	AttrCryptomatte = "cryptomatte"
)

// ===========================================
// Manifest Construction
// ===========================================

// NewManifest creates an empty manifest.
func NewManifest() *Manifest {
	return &Manifest{
		Groups: make([]ChannelGroupManifest, 0),
	}
}

// AddGroup adds a new channel group to the manifest.
func (m *Manifest) AddGroup(channels []string, components []string) *ChannelGroupManifest {
	group := ChannelGroupManifest{
		Channels:       channels,
		Components:     components,
		Lifetime:       LifetimeStable,
		HashScheme:     HashMurmur3_32,
		EncodingScheme: EncodeID,
		Entries:        make(map[uint64][]string),
	}
	m.Groups = append(m.Groups, group)
	return &m.Groups[len(m.Groups)-1]
}

// Insert adds an ID mapping with explicit ID value.
func (g *ChannelGroupManifest) Insert(id uint64, values ...string) {
	g.Entries[id] = values
}

// InsertHashed computes hash from values and inserts the mapping.
// Returns the computed ID.
func (g *ChannelGroupManifest) InsertHashed(values ...string) uint64 {
	// Join values for hashing
	key := strings.Join(values, "\x00")

	var id uint64
	switch g.HashScheme {
	case HashMurmur3_32:
		// Use CryptomatteHash which includes the denormalization fix
		// for avoiding NaN/Inf when the hash is interpreted as float32
		id = uint64(CryptomatteHash(key))
	case HashMurmur3_64:
		id, _ = MurmurHash3_x64_128([]byte(key), 0)
	default:
		// For unknown schemes, use 32-bit hash with denorm fix
		id = uint64(CryptomatteHash(key))
	}

	g.Entries[id] = values
	return id
}

// ===========================================
// Manifest Lookup
// ===========================================

// Lookup finds the text values for a given ID.
func (g *ChannelGroupManifest) Lookup(id uint64) ([]string, bool) {
	values, ok := g.Entries[id]
	return values, ok
}

// LookupChannel finds the manifest for a specific channel.
func (m *Manifest) LookupChannel(channel string) *ChannelGroupManifest {
	for i := range m.Groups {
		for _, ch := range m.Groups[i].Channels {
			if ch == channel {
				return &m.Groups[i]
			}
		}
	}
	return nil
}

// ===========================================
// Header I/O
// ===========================================

// HasManifest checks if the header contains an ID manifest.
func HasManifest(h *exr.Header) bool {
	// Check for standard ID manifest
	if attr := h.Get(AttrIDManifest); attr != nil {
		return true
	}

	// Check for Cryptomatte manifest (cryptomatte/*/manifest)
	for _, attr := range h.Attributes() {
		if strings.HasPrefix(attr.Name, AttrCryptomatte) && strings.Contains(attr.Name, "manifest") {
			return true
		}
	}

	return false
}

// GetManifest extracts the ID manifest from a header.
func GetManifest(h *exr.Header) (*Manifest, error) {
	// First try standard ID manifest attribute
	if attr := h.Get(AttrIDManifest); attr != nil {
		if data, ok := attr.Value.([]byte); ok {
			return decodeManifest(data)
		}
	}

	// Try to read Cryptomatte manifest
	manifest := NewManifest()
	cryptomattes := make(map[string]*ChannelGroupManifest)

	for _, attr := range h.Attributes() {
		if !strings.HasPrefix(attr.Name, AttrCryptomatte+"/") {
			continue
		}

		// Parse attribute name: cryptomatte/<id>/<key>
		parts := strings.Split(attr.Name, "/")
		if len(parts) != 3 {
			continue
		}

		cryptoID := parts[1]
		key := parts[2]

		if cryptomattes[cryptoID] == nil {
			group := &ChannelGroupManifest{
				Channels:       make([]string, 0),
				Components:     []string{"name"},
				Lifetime:       LifetimeStable,
				HashScheme:     HashMurmur3_32,
				EncodingScheme: EncodeID,
				Entries:        make(map[uint64][]string),
			}
			cryptomattes[cryptoID] = group
		}

		group := cryptomattes[cryptoID]

		switch key {
		case "name":
			if name, ok := attr.Value.(string); ok {
				// Add channel prefix to channels list
				group.Channels = append(group.Channels, name)
			}
		case "manifest":
			if data, ok := attr.Value.(string); ok {
				// Parse JSON manifest
				var entries map[string]string
				if err := json.Unmarshal([]byte(data), &entries); err == nil {
					for name, hexID := range entries {
						// Convert hex float to uint32
						if id, err := parseHexFloat(hexID); err == nil {
							group.Entries[uint64(id)] = []string{name}
						}
					}
				}
			}
		}
	}

	// Add all cryptomatte groups to manifest
	for _, group := range cryptomattes {
		manifest.Groups = append(manifest.Groups, *group)
	}

	if len(manifest.Groups) == 0 {
		return nil, fmt.Errorf("no ID manifest found")
	}

	return manifest, nil
}

// SetManifest stores an ID manifest in the header.
func SetManifest(h *exr.Header, m *Manifest) error {
	data, err := encodeManifest(m)
	if err != nil {
		return err
	}

	// Use custom attribute type for ID manifest data
	h.Set(&exr.Attribute{
		Name:  AttrIDManifest,
		Type:  exr.AttributeType(AttrIDManifest), // Custom type stored as raw bytes
		Value: data,
	})

	return nil
}

// ===========================================
// Cryptomatte Helpers
// ===========================================

// NewCryptomatteManifest creates a manifest suitable for Cryptomatte.
func NewCryptomatteManifest(channelPrefix string, names []string) *Manifest {
	manifest := NewManifest()

	// Cryptomatte typically uses multiple rank channels
	channels := []string{
		channelPrefix + "00.R", channelPrefix + "00.G", channelPrefix + "00.B", channelPrefix + "00.A",
		channelPrefix + "01.R", channelPrefix + "01.G", channelPrefix + "01.B", channelPrefix + "01.A",
		channelPrefix + "02.R", channelPrefix + "02.G", channelPrefix + "02.B", channelPrefix + "02.A",
	}

	group := manifest.AddGroup(channels, []string{"name"})
	group.HashScheme = HashMurmur3_32
	group.EncodingScheme = EncodeID

	for _, name := range names {
		group.InsertHashed(name)
	}

	return manifest
}

// CryptomatteHash computes the MurmurHash3 ID for a name (Cryptomatte convention).
// This returns the hash as a uint32 that can be reinterpreted as a float32.
//
// The hash is modified to avoid IEEE 754 denormalized floats, NaN, and Infinity
// by XORing bit 23 when the exponent field is 0 or 255. This matches the
// reference Python implementation in cryptomatte_utilities.py.
func CryptomatteHash(name string) uint32 {
	hash := MurmurHash3_32([]byte(name), 0)
	// Avoid denormals (exp=0) and NaN/Inf (exp=255) by flipping bit 23
	// Reference: https://github.com/Psyop/Cryptomatte/blob/master/nuke/cryptomatte_utilities.py
	exp := (hash >> 23) & 0xFF
	if exp == 0 || exp == 255 {
		hash ^= 1 << 23
	}
	return hash
}

// CryptomatteHashFloat computes the Cryptomatte hash and returns it as a float32.
// The hash is reinterpreted as a float32 (not converted), with the same
// denormalization fix applied as CryptomatteHash.
func CryptomatteHashFloat(name string) float32 {
	return math.Float32frombits(CryptomatteHash(name))
}

// SetCryptomatteManifest stores a Cryptomatte-style manifest in the header.
// This uses the standard Cryptomatte attribute naming convention.
// The output is deterministic: entries are written in alphabetical order by name.
func SetCryptomatteManifest(h *exr.Header, name string, layerIndex int, names []string) error {
	// Sort names for deterministic output
	sortedNames := make([]string, len(names))
	copy(sortedNames, names)
	sort.Strings(sortedNames)

	// Build JSON manifest with sorted keys for deterministic output
	// json.Marshal iterates maps in random order, so we build the JSON manually
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, n := range sortedNames {
		if i > 0 {
			buf.WriteByte(',')
		}
		hash := CryptomatteHash(n)
		// JSON-encode the key and value
		keyJSON, _ := json.Marshal(n)
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.WriteString(fmt.Sprintf("\"%08x\"", hash))
	}
	buf.WriteByte('}')
	manifestJSON := buf.Bytes()

	prefix := fmt.Sprintf("%s/%02d", AttrCryptomatte, layerIndex)

	// Set the manifest attribute
	h.Set(&exr.Attribute{
		Name:  prefix + "/name",
		Type:  exr.AttrTypeString,
		Value: name,
	})

	h.Set(&exr.Attribute{
		Name:  prefix + "/manifest",
		Type:  exr.AttrTypeString,
		Value: string(manifestJSON),
	})

	return nil
}

// ===========================================
// MurmurHash3 Implementation
// ===========================================

// MurmurHash3_32 computes a 32-bit MurmurHash3 hash.
// This is the standard hash used by Cryptomatte.
func MurmurHash3_32(data []byte, seed uint32) uint32 {
	const c1 = 0xcc9e2d51
	const c2 = 0x1b873593
	const r1 = 15
	const r2 = 13
	const m = 5
	const n = 0xe6546b64

	h := seed
	length := len(data)
	nblocks := length / 4

	// Body
	for i := 0; i < nblocks; i++ {
		k := binary.LittleEndian.Uint32(data[i*4:])

		k *= c1
		k = rotl32(k, r1)
		k *= c2

		h ^= k
		h = rotl32(h, r2)
		h = h*m + n
	}

	// Tail
	tail := data[nblocks*4:]
	var k uint32

	switch len(tail) {
	case 3:
		k ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k ^= uint32(tail[0])
		k *= c1
		k = rotl32(k, r1)
		k *= c2
		h ^= k
	}

	// Finalization
	h ^= uint32(length)
	h = fmix32(h)

	return h
}

// MurmurHash3_x64_128 computes a 128-bit MurmurHash3 hash.
// Returns the lower and upper 64 bits.
func MurmurHash3_x64_128(data []byte, seed uint64) (uint64, uint64) {
	const c1 uint64 = 0x87c37b91114253d5
	const c2 uint64 = 0x4cf5ad432745937f

	h1 := seed
	h2 := seed
	length := len(data)
	nblocks := length / 16

	// Body
	for i := 0; i < nblocks; i++ {
		k1 := binary.LittleEndian.Uint64(data[i*16:])
		k2 := binary.LittleEndian.Uint64(data[i*16+8:])

		k1 *= c1
		k1 = rotl64(k1, 31)
		k1 *= c2
		h1 ^= k1

		h1 = rotl64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2
		k2 = rotl64(k2, 33)
		k2 *= c1
		h2 ^= k2

		h2 = rotl64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}

	// Tail
	tail := data[nblocks*16:]
	var k1, k2 uint64

	switch len(tail) {
	case 15:
		k2 ^= uint64(tail[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(tail[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(tail[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(tail[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(tail[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(tail[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(tail[8])
		k2 *= c2
		k2 = rotl64(k2, 33)
		k2 *= c1
		h2 ^= k2
		fallthrough
	case 8:
		k1 ^= uint64(tail[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(tail[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(tail[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(tail[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(tail[0])
		k1 *= c1
		k1 = rotl64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}

	// Finalization
	h1 ^= uint64(length)
	h2 ^= uint64(length)

	h1 += h2
	h2 += h1

	h1 = fmix64(h1)
	h2 = fmix64(h2)

	h1 += h2
	h2 += h1

	return h1, h2
}

func rotl32(x uint32, r int) uint32 {
	return (x << r) | (x >> (32 - r))
}

func rotl64(x uint64, r int) uint64 {
	return (x << r) | (x >> (64 - r))
}

func fmix32(h uint32) uint32 {
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

func fmix64(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

// ===========================================
// Serialization (Internal)
// ===========================================

const manifestVersion uint32 = 1

func encodeManifest(m *Manifest) ([]byte, error) {
	var buf bytes.Buffer

	// Compress with zlib
	w := zlib.NewWriter(&buf)

	// Write version
	if err := binary.Write(w, binary.LittleEndian, manifestVersion); err != nil {
		return nil, err
	}

	// Write number of groups
	if err := binary.Write(w, binary.LittleEndian, uint32(len(m.Groups))); err != nil {
		return nil, err
	}

	for _, group := range m.Groups {
		// Write channels
		if err := writeStringList(w, group.Channels); err != nil {
			return nil, err
		}

		// Write components
		if err := writeStringList(w, group.Components); err != nil {
			return nil, err
		}

		// Write lifetime
		if err := binary.Write(w, binary.LittleEndian, uint8(group.Lifetime)); err != nil {
			return nil, err
		}

		// Write hash scheme
		if err := writeString(w, string(group.HashScheme)); err != nil {
			return nil, err
		}

		// Write encoding scheme
		if err := writeString(w, string(group.EncodingScheme)); err != nil {
			return nil, err
		}

		// Write entries
		if err := binary.Write(w, binary.LittleEndian, uint64(len(group.Entries))); err != nil {
			return nil, err
		}

		// Sort entry IDs for deterministic output
		entryIDs := make([]uint64, 0, len(group.Entries))
		for id := range group.Entries {
			entryIDs = append(entryIDs, id)
		}
		sort.Slice(entryIDs, func(i, j int) bool { return entryIDs[i] < entryIDs[j] })

		for _, id := range entryIDs {
			values := group.Entries[id]
			if err := binary.Write(w, binary.LittleEndian, id); err != nil {
				return nil, err
			}
			if err := writeStringList(w, values); err != nil {
				return nil, err
			}
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decodeManifest(data []byte) (*Manifest, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Read version
	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, err
	}

	if version != manifestVersion {
		return nil, fmt.Errorf("unsupported manifest version: %d", version)
	}

	// Read number of groups
	var numGroups uint32
	if err := binary.Read(r, binary.LittleEndian, &numGroups); err != nil {
		return nil, err
	}

	manifest := &Manifest{
		Groups: make([]ChannelGroupManifest, numGroups),
	}

	for i := range manifest.Groups {
		group := &manifest.Groups[i]

		// Read channels
		channels, err := readStringList(r)
		if err != nil {
			return nil, err
		}
		group.Channels = channels

		// Read components
		components, err := readStringList(r)
		if err != nil {
			return nil, err
		}
		group.Components = components

		// Read lifetime
		var lifetime uint8
		if err := binary.Read(r, binary.LittleEndian, &lifetime); err != nil {
			return nil, err
		}
		group.Lifetime = IDLifetime(lifetime)

		// Read hash scheme
		hashScheme, err := readString(r)
		if err != nil {
			return nil, err
		}
		group.HashScheme = HashScheme(hashScheme)

		// Read encoding scheme
		encScheme, err := readString(r)
		if err != nil {
			return nil, err
		}
		group.EncodingScheme = EncodingScheme(encScheme)

		// Read entries
		var numEntries uint64
		if err := binary.Read(r, binary.LittleEndian, &numEntries); err != nil {
			return nil, err
		}

		group.Entries = make(map[uint64][]string)

		for j := uint64(0); j < numEntries; j++ {
			var id uint64
			if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
				return nil, err
			}

			values, err := readStringList(r)
			if err != nil {
				return nil, err
			}

			group.Entries[id] = values
		}
	}

	return manifest, nil
}

func writeString(w io.Writer, s string) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(s))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s))
	return err
}

func readString(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	// Limit string length to prevent DoS (max 16MB per string)
	const maxStringLength = 16 * 1024 * 1024
	if length > maxStringLength {
		return "", fmt.Errorf("exrid: string length %d exceeds maximum %d", length, maxStringLength)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}

	return string(data), nil
}

func writeStringList(w io.Writer, list []string) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(list))); err != nil {
		return err
	}

	for _, s := range list {
		if err := writeString(w, s); err != nil {
			return err
		}
	}

	return nil
}

func readStringList(r io.Reader) ([]string, error) {
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, err
	}

	// Limit list size to prevent DoS (max 1M entries)
	const maxListSize = 1024 * 1024
	if count > maxListSize {
		return nil, fmt.Errorf("exrid: list count %d exceeds maximum %d", count, maxListSize)
	}

	list := make([]string, count)
	for i := range list {
		s, err := readString(r)
		if err != nil {
			return nil, err
		}
		list[i] = s
	}

	return list, nil
}

// parseHexFloat parses a hex string as a uint32 (used for Cryptomatte manifest parsing).
func parseHexFloat(s string) (uint32, error) {
	var val uint32
	_, err := fmt.Sscanf(s, "%x", &val)
	return val, err
}
