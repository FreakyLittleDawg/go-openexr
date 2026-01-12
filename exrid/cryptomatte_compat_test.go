package exrid

import (
	"fmt"
	"math"
	"testing"
)

// TestCryptomatteHashDenormalization tests that CryptomatteHash properly handles
// IEEE 754 denormalized floats and special values (NaN/Inf) by XORing bit 23
// when the exponent field is 0 or 255.
//
// Reference: https://github.com/Psyop/Cryptomatte/blob/master/nuke/cryptomatte_utilities.py
// The Python mm3hash_float function does:
//
//	exp = hash_32 >> 23 & 255
//	if (exp == 0) or (exp == 255):
//	    hash_32 ^= 1 << 23
func TestCryptomatteHashDenormalization(t *testing.T) {
	// These names from real Cryptomatte files produce hashes with exponent 255
	// Our implementation differs from the reference by exactly bit 23 (0x00800000)
	testCases := []struct {
		name       string
		storedID   uint32 // ID as stored in the reference Cryptomatte file
		computedID uint32 // What our current implementation produces
		xorDiff    uint32 // Expected XOR difference (should be 0x00800000)
	}{
		{
			name:       "flowerB.flowerB_petal16.:3106",
			storedID:   2133727083, // 0x7F2E176B
			computedID: 2142115691, // 0x7FAE176B
			xorDiff:    0x00800000,
		},
		{
			name:       "plant.smallStalk1.:3440",
			storedID:   2158911662, // 0x80AE60AE
			computedID: 2150523054, // 0x802E60AE
			xorDiff:    0x00800000,
		},
		{
			name:       "plant.smallStalk1.:1900",
			storedID:   2135305282, // 0x7F462C42
			computedID: 2143693890, // 0x7FC62C42
			xorDiff:    0x00800000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify our current implementation produces the computedID
			hash := MurmurHash3_32([]byte(tc.name), 0)
			if hash != tc.computedID {
				t.Errorf("MurmurHash3_32(%q) = 0x%08X, want 0x%08X", tc.name, hash, tc.computedID)
			}

			// Verify the XOR difference is exactly bit 23
			diff := tc.storedID ^ tc.computedID
			if diff != tc.xorDiff {
				t.Errorf("XOR diff = 0x%08X, want 0x%08X", diff, tc.xorDiff)
			}

			// Check that computed hash has exponent 255 (needs denorm fix)
			exp := (hash >> 23) & 0xFF
			if exp != 255 && exp != 0 {
				t.Logf("Hash 0x%08X has exponent %d (not a denorm case)", hash, exp)
			}

			// After denorm fix, the stored ID should have a valid exponent
			storedExp := (tc.storedID >> 23) & 0xFF
			if storedExp == 0 || storedExp == 255 {
				t.Errorf("Stored ID 0x%08X still has invalid exponent %d after fix", tc.storedID, storedExp)
			}

			// The CryptomatteHash function SHOULD return storedID (after fix is applied)
			// This test will FAIL until we fix CryptomatteHash
			cryptoHash := CryptomatteHash(tc.name)
			if cryptoHash != tc.storedID {
				t.Errorf("CryptomatteHash(%q) = 0x%08X (exp=%d), want 0x%08X (exp=%d)\n"+
					"  -> BUG: Missing denormalization fix (XOR bit 23)",
					tc.name, cryptoHash, (cryptoHash>>23)&0xFF,
					tc.storedID, storedExp)
			}
		})
	}
}

// TestCryptomatteHashFloatReferenceVectors tests against the Python reference implementation.
// These test vectors are from cryptomatte_utilities_tests.py
//
// Reference: https://github.com/Psyop/Cryptomatte/blob/master/nuke/cryptomatte_utilities_tests.py
func TestCryptomatteHashFloatReferenceVectors(t *testing.T) {
	// Test vectors from Python cryptomatte_utilities_tests.py CryptoHashing class
	testCases := []struct {
		name     string
		expected float32
	}{
		{"hello", 6.0705627102400005616e-17},
		{"cube", -4.08461912519e+15},
		{"sphere", 2.79018604383e+15},
		{"plane", 3.66557617593e-11},
		// Note: UTF-8 test cases would require exact string encoding
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CryptomatteHashFloat(tc.name)

			// Compare with tolerance for floating point
			// The Python test uses exact comparison, but we allow small epsilon
			diff := math.Abs(float64(result) - float64(tc.expected))
			relDiff := diff / math.Abs(float64(tc.expected))

			if relDiff > 1e-6 {
				t.Errorf("CryptomatteHashFloat(%q) = %e, want %e (rel diff: %e)",
					tc.name, result, tc.expected, relDiff)
			}
		})
	}
}

// TestCryptomatteHashFloatNeverNaNOrInf verifies that CryptomatteHashFloat
// never returns NaN or Infinity values after the denormalization fix.
func TestCryptomatteHashFloatNeverNaNOrInf(t *testing.T) {
	// Test a variety of strings that might produce problematic hashes
	testNames := []string{
		"hello",
		"world",
		"Hero",
		"Villain",
		"flowerB.flowerB_petal16.:3106",
		"plant.smallStalk1.:3440",
		"object_001",
		"cube",
		"sphere",
		"plane",
		"test123",
		"",
		"a",
		"ab",
		"abc",
		"abcd",
		"This is a longer object name with spaces",
		"path/to/object",
		"namespace:object",
		"объект", // Russian word for "object"
	}

	for _, name := range testNames {
		result := CryptomatteHashFloat(name)

		if math.IsNaN(float64(result)) {
			t.Errorf("CryptomatteHashFloat(%q) returned NaN", name)
		}
		if math.IsInf(float64(result), 0) {
			t.Errorf("CryptomatteHashFloat(%q) returned Inf", name)
		}
	}
}

// TestExponentBit23Fix verifies the bit 23 XOR logic for denormalization
func TestExponentBit23Fix(t *testing.T) {
	// Test case: hash that produces exponent 255 (0xFF)
	// After XOR with 0x00800000, exponent should be 254 (0xFE)
	testCases := []struct {
		hash        uint32
		expectedExp uint32
		needsFix    bool
	}{
		{0x7FFFFFFF, 254, true},  // exp=255 -> 254 after fix
		{0xFF800000, 254, true},  // exp=255 -> 254 after fix
		{0x00400000, 1, true},    // exp=0 -> 1 after fix
		{0x00000001, 1, true},    // exp=0 -> 1 after fix
		{0x3F800000, 127, false}, // exp=127, no fix needed
		{0x40000000, 128, false}, // exp=128, no fix needed
	}

	for _, tc := range testCases {
		exp := (tc.hash >> 23) & 0xFF
		needsFix := exp == 0 || exp == 255

		if needsFix != tc.needsFix {
			t.Errorf("hash 0x%08X: needsFix=%v, want %v", tc.hash, needsFix, tc.needsFix)
		}

		// Apply fix
		fixed := tc.hash
		if needsFix {
			fixed ^= 1 << 23
		}

		fixedExp := (fixed >> 23) & 0xFF
		if fixedExp != tc.expectedExp {
			t.Errorf("hash 0x%08X after fix: exp=%d, want %d", tc.hash, fixedExp, tc.expectedExp)
		}
	}
}

// TestMurmurHash3SignedUnsignedConsistency verifies our hash matches the Python
// mmh3 library behavior. Python mmh3.hash() returns signed int32, but we use uint32.
// The manifest stores hex values which are unsigned representations.
func TestMurmurHash3SignedUnsignedConsistency(t *testing.T) {
	// The Python mmh3.hash() returns signed values, but when packed with
	// struct.pack('<L', hash_32 & 0xffffffff), it becomes unsigned.
	// Our Go implementation should produce the same unsigned values.

	testCases := []struct {
		input    string
		expected uint32 // Expected unsigned hash value
	}{
		{"", 0},
		{"hello", 0x248BFA47},
		{"Hello, world!", 0xc0363e43},
	}

	for _, tc := range testCases {
		result := MurmurHash3_32([]byte(tc.input), 0)
		if result != tc.expected {
			t.Errorf("MurmurHash3_32(%q, 0) = 0x%08X, want 0x%08X", tc.input, result, tc.expected)
		}
	}
}

// TestManifestHexEncodingConsistency verifies that hashes are encoded to hex
// consistently with the Python reference implementation.
func TestManifestHexEncodingConsistency(t *testing.T) {
	// In Python, the manifest stores hashes as 8-character lowercase hex strings
	// Example: {"objectName": "0a1b2c3d"}

	testCases := []struct {
		name        string
		expectedHex string // What should be in manifest JSON
	}{
		{"hello", "248bfa47"},
		{"Hero", "d3fcd9ab"}, // Verified with Python mmh3
	}

	for _, tc := range testCases {
		hash := CryptomatteHash(tc.name)
		hexStr := fmt.Sprintf("%08x", hash)

		// Verify exact match with Python mmh3 reference
		if hexStr != tc.expectedHex {
			t.Errorf("CryptomatteHash(%q) = %q, want %q (Python mmh3 reference)",
				tc.name, hexStr, tc.expectedHex)
		}

		// The hex encoding itself should be correct format
		if len(hexStr) != 8 {
			t.Errorf("Hex encoding of %q has length %d, want 8", tc.name, len(hexStr))
		}
	}
}

// BenchmarkCryptomatteHash benchmarks the hash function
func BenchmarkCryptomatteHash(b *testing.B) {
	names := []string{
		"short",
		"medium_length_name",
		"this_is_a_very_long_object_name_with_many_characters_and_path_separators/subobject/leaf",
	}

	for _, name := range names {
		end := 10
		if len(name) < end {
			end = len(name)
		}
		b.Run(name[:end], func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = CryptomatteHash(name)
			}
		})
	}
}
