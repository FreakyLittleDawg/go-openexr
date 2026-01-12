package exrid

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrjoshuak/go-openexr/exr"
)

// testdataDir returns the path to the testdata directory.
// Returns empty string if testdata is not available.
func testdataDir() string {
	// Try relative path from package directory
	candidates := []string{
		"../testdata",
		"testdata",
	}

	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return ""
}

// skipIfNoTestData skips the test if external test data is not available.
func skipIfNoTestData(t *testing.T) string {
	t.Helper()
	dir := testdataDir()
	if dir == "" {
		t.Skip("External test data not available. Run 'testdata/download.sh' to download.")
	}
	return dir
}

// skipIfFileNotExists skips the test if the specified file doesn't exist.
func skipIfFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s. Run 'testdata/download.sh' to download.", path)
	}
}

func TestParseCryptomatteFile_BunnyObject(t *testing.T) {
	dir := skipIfNoTestData(t)
	path := filepath.Join(dir, "cryptomatte", "bunny_CryptoObject.exr")
	skipIfFileNotExists(t, path)

	// Open the file
	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	h := f.Header(0)

	// Verify Cryptomatte metadata exists
	if !HasManifest(h) {
		t.Fatal("HasManifest() returned false for Cryptomatte file")
	}

	// Parse the manifest
	manifest, err := GetManifest(h)
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}

	// Verify manifest was parsed
	if manifest == nil {
		t.Fatal("GetManifest() returned nil manifest")
	}

	if len(manifest.Groups) == 0 {
		t.Fatal("Manifest has no groups")
	}

	// Check the first group
	group := &manifest.Groups[0]
	t.Logf("Group channels: %v", group.Channels)
	t.Logf("Group components: %v", group.Components)
	t.Logf("Group hash scheme: %v", group.HashScheme)
	t.Logf("Number of entries: %d", len(group.Entries))

	// Verify hash scheme is MurmurHash3_32 (standard for Cryptomatte)
	if group.HashScheme != HashMurmur3_32 {
		t.Errorf("HashScheme = %v, want HashMurmur3_32", group.HashScheme)
	}

	// Verify we have some entries
	if len(group.Entries) == 0 {
		t.Error("Manifest has no entries")
	}

	// Log some sample entries
	count := 0
	for id, values := range group.Entries {
		if count < 5 {
			t.Logf("  Entry: ID=%d, Values=%v", id, values)
		}
		count++
	}
	t.Logf("Total entries: %d", count)
}

func TestParseCryptomatteFile_BunnyMaterial(t *testing.T) {
	dir := skipIfNoTestData(t)
	path := filepath.Join(dir, "cryptomatte", "bunny_CryptoMaterial.exr")
	skipIfFileNotExists(t, path)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	h := f.Header(0)

	if !HasManifest(h) {
		t.Fatal("HasManifest() returned false for Cryptomatte file")
	}

	manifest, err := GetManifest(h)
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}

	if len(manifest.Groups) == 0 {
		t.Fatal("Manifest has no groups")
	}

	t.Logf("Material manifest has %d entries", len(manifest.Groups[0].Entries))
}

func TestParseCryptomatteFile_BunnyAsset(t *testing.T) {
	dir := skipIfNoTestData(t)
	path := filepath.Join(dir, "cryptomatte", "bunny_CryptoAsset.exr")
	skipIfFileNotExists(t, path)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	h := f.Header(0)

	if !HasManifest(h) {
		t.Fatal("HasManifest() returned false for Cryptomatte file")
	}

	manifest, err := GetManifest(h)
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}

	if len(manifest.Groups) == 0 {
		t.Fatal("Manifest has no groups")
	}

	t.Logf("Asset manifest has %d entries", len(manifest.Groups[0].Entries))
}

func TestParseCryptomatteFile_TestGrid(t *testing.T) {
	dir := skipIfNoTestData(t)
	path := filepath.Join(dir, "cryptomatte", "testGrid_CryptoObject.exr")
	skipIfFileNotExists(t, path)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	h := f.Header(0)

	if !HasManifest(h) {
		t.Fatal("HasManifest() returned false for Cryptomatte file")
	}

	manifest, err := GetManifest(h)
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}

	if len(manifest.Groups) == 0 {
		t.Fatal("Manifest has no groups")
	}

	t.Logf("TestGrid manifest has %d entries", len(manifest.Groups[0].Entries))
}

func TestCryptomatteHashVerification(t *testing.T) {
	dir := skipIfNoTestData(t)
	path := filepath.Join(dir, "cryptomatte", "bunny_CryptoObject.exr")
	skipIfFileNotExists(t, path)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	manifest, err := GetManifest(f.Header(0))
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}

	if len(manifest.Groups) == 0 {
		t.Fatal("No groups in manifest")
	}

	group := &manifest.Groups[0]

	// For each entry, verify the hash matches what we'd compute
	verified := 0
	mismatched := 0
	for id, values := range group.Entries {
		if len(values) == 0 {
			continue
		}
		name := values[0]

		// Compute expected hash
		expectedHash := uint64(CryptomatteHash(name))

		if id == expectedHash {
			verified++
		} else {
			mismatched++
			if mismatched <= 3 {
				t.Logf("Hash mismatch for %q: got %d, computed %d", name, id, expectedHash)
			}
		}
	}

	t.Logf("Hash verification: %d matched, %d mismatched out of %d total",
		verified, mismatched, len(group.Entries))

	// In a valid Cryptomatte file, hashes should match
	if verified == 0 && len(group.Entries) > 0 {
		t.Error("No hashes matched - manifest parsing may be incorrect")
	}
}

func TestLookupByName(t *testing.T) {
	dir := skipIfNoTestData(t)
	path := filepath.Join(dir, "cryptomatte", "bunny_CryptoObject.exr")
	skipIfFileNotExists(t, path)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	manifest, err := GetManifest(f.Header(0))
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}

	if len(manifest.Groups) == 0 {
		t.Fatal("No groups in manifest")
	}

	group := &manifest.Groups[0]

	// Get a sample name from the manifest
	var sampleName string
	var sampleID uint64
	for id, values := range group.Entries {
		if len(values) > 0 {
			sampleName = values[0]
			sampleID = id
			break
		}
	}

	if sampleName == "" {
		t.Skip("No entries with names in manifest")
	}

	t.Logf("Looking up entry: name=%q, id=%d", sampleName, sampleID)

	// Look up by ID
	values, found := group.Lookup(sampleID)
	if !found {
		t.Errorf("Lookup(%d) not found", sampleID)
	} else if len(values) == 0 || values[0] != sampleName {
		t.Errorf("Lookup(%d) = %v, want [%q]", sampleID, values, sampleName)
	}
}
