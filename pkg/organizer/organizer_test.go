package organizer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// organizer_test.go verifies organizer cleanup and DB tracking behavior.

func TestRun_RemovesOrganizedWhenSourceMissing(t *testing.T) {
	baseDir := t.TempDir()

	libraryDir := filepath.Join(baseDir, "library")
	organizedDir := filepath.Join(baseDir, "library-organized")
	cacheDir := filepath.Join(baseDir, "cache")

	if err := os.MkdirAll(organizedDir, 0755); err != nil {
		t.Fatalf("mkdir organized: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}

	relPath := filepath.Join("Movies", "Example (2024)", "Example (2024).strm")
	destFullPath := filepath.Join(organizedDir, relPath)
	if err := os.MkdirAll(filepath.Dir(destFullPath), 0755); err != nil {
		t.Fatalf("mkdir dest dir: %v", err)
	}
	if err := os.WriteFile(destFullPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write organized file: %v", err)
	}

	// Tracking entry exists but source file is missing.
	trackingPath := filepath.Join(cacheDir, "file_tracking.json")
	tracking := map[string]TrackingEntry{
		relPath: {Link: "https://real-debrid.com/d/ABC123"},
	}
	trackingBytes, err := json.Marshal(tracking)
	if err != nil {
		t.Fatalf("marshal tracking: %v", err)
	}
	if err := os.WriteFile(trackingPath, trackingBytes, 0644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}

	// Organizer DB references the organized file.
	dbPath := filepath.Join(cacheDir, "organizer_db.json")
	db := map[string]FileEntry{
		relPath: {DestPath: relPath, RDID: "ABC123", Type: "movie"},
	}
	dbBytes, err := json.Marshal(db)
	if err != nil {
		t.Fatalf("marshal db: %v", err)
	}
	if err := os.WriteFile(dbPath, dbBytes, 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}

	org := New(Config{
		BaseDir:      baseDir,
		OrganizedDir: organizedDir,
		OutputDir:    libraryDir,
		TrackingFile: trackingPath,
		CacheDir:     cacheDir,
		Logger:       zerolog.Nop(),
	})

	result := org.Run()
	if result.Deleted != 1 {
		t.Fatalf("expected 1 deleted file, got %d", result.Deleted)
	}
	if _, err := os.Stat(destFullPath); err == nil {
		t.Fatalf("expected organized file to be removed")
	}
}

func TestRun_UpdatesOrganizedWhenDownloadURLChanges(t *testing.T) {
	baseDir := t.TempDir()

	libraryDir := filepath.Join(baseDir, "library")
	organizedDir := filepath.Join(baseDir, "library-organized")
	cacheDir := filepath.Join(baseDir, "cache")

	if err := os.MkdirAll(libraryDir, 0755); err != nil {
		t.Fatalf("mkdir library: %v", err)
	}
	if err := os.MkdirAll(organizedDir, 0755); err != nil {
		t.Fatalf("mkdir organized: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}

	relPath := filepath.Join("Movies", "Example (2024)", "Example (2024).strm")
	sourceFullPath := filepath.Join(libraryDir, relPath)
	destFullPath := filepath.Join(organizedDir, relPath)

	if err := os.MkdirAll(filepath.Dir(sourceFullPath), 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destFullPath), 0755); err != nil {
		t.Fatalf("mkdir dest dir: %v", err)
	}

	newURL := "https://new.example/stream"
	oldURL := "https://old.example/stream"

	if err := os.WriteFile(sourceFullPath, []byte(newURL), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(destFullPath, []byte(oldURL), 0644); err != nil {
		t.Fatalf("write dest file: %v", err)
	}

	// Tracking entry contains the new URL.
	trackingPath := filepath.Join(cacheDir, "file_tracking.json")
	tracking := map[string]TrackingEntry{
		relPath: {Link: "https://real-debrid.com/d/ABC123", DownloadURL: newURL},
	}
	trackingBytes, err := json.Marshal(tracking)
	if err != nil {
		t.Fatalf("marshal tracking: %v", err)
	}
	if err := os.WriteFile(trackingPath, trackingBytes, 0644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}

	// Organizer DB references the organized file with the old URL.
	dbPath := filepath.Join(cacheDir, "organizer_db.json")
	db := map[string]FileEntry{
		relPath: {DestPath: relPath, RDID: "ABC123", Type: "movie", DownloadURL: oldURL},
	}
	dbBytes, err := json.Marshal(db)
	if err != nil {
		t.Fatalf("marshal db: %v", err)
	}
	if err := os.WriteFile(dbPath, dbBytes, 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}

	org := New(Config{
		BaseDir:      baseDir,
		OrganizedDir: organizedDir,
		OutputDir:    libraryDir,
		TrackingFile: trackingPath,
		CacheDir:     cacheDir,
		Logger:       zerolog.Nop(),
	})

	result := org.Run()
	if result.New != 1 {
		t.Fatalf("expected 1 updated file to be copied, got new=%d updated=%d", result.New, result.Updated)
	}

	updatedBytes, err := os.ReadFile(destFullPath)
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}
	if string(updatedBytes) != newURL {
		t.Fatalf("expected organized file to be updated with new URL")
	}
}

func TestRun_UsesCWDForRelativeOrganizedDir(t *testing.T) {
	rootDir := t.TempDir()
	appDir := filepath.Join(rootDir, "app")
	configDir := filepath.Join(rootDir, "data")

	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore wd: %v", chdirErr)
		}
	}()

	if err := os.Chdir(appDir); err != nil {
		t.Fatalf("chdir app: %v", err)
	}

	relPath := filepath.Join("Some.Movie.2024", "Some.Movie.2024.strm")
	sourceFullPath := filepath.Join("library", relPath)
	if err := os.MkdirAll(filepath.Dir(sourceFullPath), 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourceFullPath, []byte("https://new.example/stream"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	trackingPath := filepath.Join("cache", "file_tracking.json")
	if err := os.MkdirAll(filepath.Dir(trackingPath), 0755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	tracking := map[string]TrackingEntry{
		relPath: {Link: "https://real-debrid.com/d/ABC123", DownloadURL: "https://new.example/stream"},
	}
	trackingBytes, err := json.Marshal(tracking)
	if err != nil {
		t.Fatalf("marshal tracking: %v", err)
	}
	if err := os.WriteFile(trackingPath, trackingBytes, 0644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}

	org := New(Config{
		BaseDir:      configDir,
		OrganizedDir: "./library-organized",
		OutputDir:    "./library",
		TrackingFile: trackingPath,
		CacheDir:     "./cache",
		Logger:       zerolog.Nop(),
	})

	result := org.Run()
	if result.New != 1 {
		t.Fatalf("expected 1 organized file, got new=%d", result.New)
	}

	entry, exists := org.db[relPath]
	if !exists {
		t.Fatalf("expected organizer DB entry for %s", relPath)
	}

	appDest := filepath.Join(appDir, "library-organized", entry.DestPath)
	if _, err := os.Stat(appDest); err != nil {
		t.Fatalf("expected organized file in app directory, got err=%v", err)
	}

	configDest := filepath.Join(configDir, "library-organized", entry.DestPath)
	if _, err := os.Stat(configDest); err == nil {
		t.Fatalf("did not expect organized file under config directory: %s", configDest)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error for config directory target: %v", err)
	}
}

func TestRun_PrunesOrphanEmptyFolders(t *testing.T) {
	baseDir := t.TempDir()

	libraryDir := filepath.Join(baseDir, "library")
	organizedDir := filepath.Join(baseDir, "library-organized")
	cacheDir := filepath.Join(baseDir, "cache")

	if err := os.MkdirAll(libraryDir, 0755); err != nil {
		t.Fatalf("mkdir library: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}

	// Create an orphan empty tree that is not tied to organizer DB/tracking cleanup.
	orphanEmptyDir := filepath.Join(organizedDir, "Series", "Orphan Show (2026)", "Season 01")
	if err := os.MkdirAll(orphanEmptyDir, 0755); err != nil {
		t.Fatalf("mkdir orphan empty dir: %v", err)
	}

	// Create one tracked source file so Run() performs normal work.
	relPath := filepath.Join("Some.Movie.2024", "Some.Movie.2024.strm")
	sourceFullPath := filepath.Join(libraryDir, relPath)
	if err := os.MkdirAll(filepath.Dir(sourceFullPath), 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourceFullPath, []byte("https://example.test/stream"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	trackingPath := filepath.Join(cacheDir, "file_tracking.json")
	tracking := map[string]TrackingEntry{
		relPath: {Link: "https://real-debrid.com/d/ABC123", DownloadURL: "https://example.test/stream"},
	}
	trackingBytes, err := json.Marshal(tracking)
	if err != nil {
		t.Fatalf("marshal tracking: %v", err)
	}
	if err := os.WriteFile(trackingPath, trackingBytes, 0644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}

	org := New(Config{
		BaseDir:      baseDir,
		OrganizedDir: organizedDir,
		OutputDir:    libraryDir,
		TrackingFile: trackingPath,
		CacheDir:     cacheDir,
		Logger:       zerolog.Nop(),
	})

	_ = org.Run()

	if _, err := os.Stat(orphanEmptyDir); !os.IsNotExist(err) {
		t.Fatalf("expected orphan empty directory to be pruned, got err=%v", err)
	}
}

func TestStripSourcePrefix(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "www 1Source gs Example Title", want: "Example Title"},
		{in: "www 1Source tag Example of Words", want: "Example of Words"},
		{in: "www SomeIndex org Sample and Test", want: "Sample and Test"},
		{in: "www NewSource com Generic Movie", want: "Generic Movie"},
		{in: "Plain Title", want: "Plain Title"},
	}

	for _, tc := range tests {
		got := stripSourcePrefix(tc.in)
		if got != tc.want {
			t.Fatalf("stripSourcePrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGetContentTypeAndPath_StripsSourcePrefixForMovie(t *testing.T) {
	org := New(Config{
		BaseDir: t.TempDir(),
		Logger:  zerolog.Nop(),
	})

	parsed := org.parser.Parse("www 1Source gs Generic Movie (2026)")
	contentType, dest := org.getContentTypeAndPath(parsed, nil, "www 1Source gs Generic Movie (2026).strm", "ABCDEF123456")
	if contentType != "movie" {
		t.Fatalf("expected movie type, got %s", contentType)
	}

	expected := filepath.Join("Movies", "Generic Movie (2026)", "Generic Movie (2026).strm")
	if dest != expected {
		t.Fatalf("unexpected destination path: got %q want %q", dest, expected)
	}
}
