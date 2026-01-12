package exrid

import (
	"testing"
)

// FuzzMurmur3Hash tests the MurmurHash3 implementation.
func FuzzMurmur3Hash(f *testing.F) {
	f.Add("")
	f.Add("object")
	f.Add("Object.001")
	f.Add("very_long_object_name_with_many_characters")
	f.Add("\x00\x01\x02\x03")          // Binary garbage
	f.Add("日本語オブジェクト")                 // Unicode
	f.Add(string(make([]byte, 10000))) // Very long string

	f.Fuzz(func(t *testing.T, name string) {
		// Hash generation should not panic
		hash := MurmurHash3_32([]byte(name), 0)
		_ = hash

		// Hash to float should not panic (via Cryptomatte method)
		id := CryptomatteHashFloat(name)
		_ = id
	})
}

// FuzzManifest tests manifest operations.
func FuzzManifest(f *testing.F) {
	// Add seed channel/component combinations
	f.Add("objectId.R", "object")
	f.Add("cryptomatte/00/R", "crypto")
	f.Add("", "")
	f.Add("channel.with.dots", "comp.name")

	f.Fuzz(func(t *testing.T, channelName, componentName string) {
		// Create a manifest
		manifest := NewManifest()

		// Add a group - should not panic
		channels := []string{channelName}
		components := []string{componentName}

		group := manifest.AddGroup(channels, components)
		if group == nil {
			return
		}

		// Insert some entries
		group.InsertHashed("TestObject")
		group.InsertHashed("AnotherObject")
		group.InsertHashed(channelName) // Use input as object name too

		// Lookup should not panic
		entries := group.Entries
		_ = entries
	})
}

// FuzzChannelGroupManifest tests channel group operations.
func FuzzChannelGroupManifest(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(0xFFFFFFFFFFFFFFFF))
	f.Add(uint64(12345678))

	f.Fuzz(func(t *testing.T, id uint64) {
		group := &ChannelGroupManifest{
			Channels:   []string{"R", "G"},
			Components: []string{"object"},
			Entries:    make(map[uint64][]string),
		}

		// Insert with arbitrary ID
		group.Entries[id] = []string{"TestObject"}

		// Lookup should not panic
		val := group.Entries[id]
		_ = val

		// Check if entry exists (alternative to GetName)
		if names, ok := group.Entries[id]; ok {
			_ = names
		}
	})
}

// FuzzInsertHashed tests the InsertHashed function with various names.
func FuzzInsertHashed(f *testing.F) {
	f.Add("simple")
	f.Add("")
	f.Add("name with spaces")
	f.Add("name/with/slashes")
	f.Add("name\x00with\x00nulls")
	f.Add(string(make([]byte, 1000)))

	f.Fuzz(func(t *testing.T, name string) {
		group := &ChannelGroupManifest{
			Channels:   []string{"R"},
			Components: []string{"object"},
			HashScheme: HashMurmur3_32,
			Entries:    make(map[uint64][]string),
		}

		// InsertHashed should not panic
		group.InsertHashed(name)

		// Verify the entry exists
		found := false
		for _, values := range group.Entries {
			for _, v := range values {
				if v == name {
					found = true
					break
				}
			}
		}

		if name != "" && !found {
			t.Errorf("inserted name not found in entries")
		}
	})
}
