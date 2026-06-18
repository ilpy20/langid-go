package langid

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSetLanguagesAndReset(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	origCount := id.activeRuntime().numLangs
	if origCount == 0 {
		t.Fatalf("expected positive number of original languages, got 0")
	}

	// 1. Restrict to a valid subset ("en", "fr")
	err = id.SetLanguages("en", "fr")
	if err != nil {
		t.Fatalf("SetLanguages(\"en\", \"fr\") failed: %v", err)
	}

	if got := id.activeRuntime().numLangs; got != 2 {
		t.Errorf("expected 2 active languages, got %d", got)
	}

	if got := len(id.activeRuntime().classes); got != 2 {
		t.Errorf("expected 2 active classes, got %d", got)
	}

	// Verify that the active classes are indeed "en" and "fr"
	classesMap := make(map[string]bool)
	for _, c := range id.activeRuntime().classes {
		classesMap[c] = true
	}
	if !classesMap["en"] || !classesMap["fr"] {
		t.Errorf("active classes do not match [en, fr]: %v", id.activeRuntime().classes)
	}

	// German text ("das ist ein deutscher satz") must classify as either "en" or "fr" now
	res, err := id.IdentifyString("das ist ein deutscher satz")
	if err != nil {
		t.Fatalf("IdentifyString failed: %v", err)
	}
	if res.Language != "en" && res.Language != "fr" {
		t.Errorf("expected German text to be classified as en/fr under subset restriction, got %q", res.Language)
	}

	// 2. Restrict to another valid subset ("de", "it") to test multiple restrictions
	err = id.SetLanguages("de", "it")
	if err != nil {
		t.Fatalf("SetLanguages(\"de\", \"it\") failed: %v", err)
	}

	if got := id.activeRuntime().numLangs; got != 2 {
		t.Errorf("expected 2 active languages, got %d", got)
	}

	classesMap2 := make(map[string]bool)
	for _, c := range id.activeRuntime().classes {
		classesMap2[c] = true
	}
	if !classesMap2["de"] || !classesMap2["it"] {
		t.Errorf("active classes do not match [de, it]: %v", id.activeRuntime().classes)
	}

	// German text should now classify as "de"
	res, err = id.IdentifyString("das ist ein deutscher satz")
	if err != nil {
		t.Fatalf("IdentifyString failed: %v", err)
	}
	if res.Language != "de" {
		t.Errorf("expected German text to classify as de, got %q", res.Language)
	}

	// 3. Reset languages back to full set
	id.ResetLanguages()

	if got := id.activeRuntime().numLangs; got != origCount {
		t.Errorf("expected restored count %d, got %d", origCount, got)
	}

	// German text should now classify as "de" still, but other languages are available
	res, err = id.IdentifyString("ceci est une phrase en francais")
	if err != nil {
		t.Fatalf("IdentifyString failed: %v", err)
	}
	if res.Language != "fr" {
		t.Errorf("expected French text to classify as fr after reset, got %q", res.Language)
	}
}

func TestSetLanguagesInvalid(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	origCount := id.activeRuntime().numLangs

	// 1. Partially invalid request
	err = id.SetLanguages("en", "invalid_lang_code", "fr")
	if err == nil {
		t.Fatalf("expected SetLanguages with partially invalid language to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), `"invalid_lang_code" is not supported`) {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify atomicity (state is unchanged)
	if got := id.activeRuntime().numLangs; got != origCount {
		t.Errorf("expected identifier state to remain unchanged (count=%d) after partial failure, but got count=%d", origCount, got)
	}

	// 2. Fully invalid request
	err = id.SetLanguages("invalid_one", "invalid_two")
	if err == nil {
		t.Fatalf("expected SetLanguages with fully invalid languages to fail, but it succeeded")
	}

	if got := id.activeRuntime().numLangs; got != origCount {
		t.Errorf("expected identifier state to remain unchanged (count=%d) after full failure, but got count=%d", origCount, got)
	}

	// 3. Empty slice / nil parameter resets languages
	err = id.SetLanguages("en", "fr")
	if err != nil {
		t.Fatalf("failed to subset languages: %v", err)
	}

	err = id.SetLanguages()
	if err != nil {
		t.Fatalf("SetLanguages() with empty slice failed: %v", err)
	}
	if got := id.activeRuntime().numLangs; got != origCount {
		t.Errorf("expected empty SetLanguages() to reset to full set (%d), got %d", origCount, got)
	}
}

func TestKeepOnlyBackwardCompatibility(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	origCount := id.activeRuntime().numLangs

	// 1. KeepOnly should succeed on valid subset
	err = id.KeepOnly("en", "fr")
	if err != nil {
		t.Fatalf("KeepOnly failed: %v", err)
	}
	if got := id.activeRuntime().numLangs; got != 2 {
		t.Errorf("expected 2 languages kept, got %d", got)
	}

	// 2. KeepOnly should fail on partially invalid subset (strict validation parity)
	err = id.KeepOnly("en", "invalid")
	if err == nil {
		t.Fatalf("expected KeepOnly with partially invalid language to fail")
	}

	// 3. KeepOnly should fail on empty args
	err = id.KeepOnly()
	if err == nil {
		t.Fatalf("expected KeepOnly with no args to fail")
	}

	// Reset languages
	id.ResetLanguages()
	if got := id.activeRuntime().numLangs; got != origCount {
		t.Errorf("expected reset to work, got %d vs %d", got, origCount)
	}
}

func TestClassificationAndRankingWithSubsets(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	// Restrict to "en", "es", "it"
	err = id.SetLanguages("en", "es", "it")
	if err != nil {
		t.Fatalf("SetLanguages failed: %v", err)
	}

	// Test IdentifyString
	res, err := id.IdentifyString("this is english")
	if err != nil {
		t.Fatalf("IdentifyString: %v", err)
	}
	if res.Language != "en" {
		t.Errorf("expected en, got %q", res.Language)
	}

	// Test RankString
	ranks, err := id.RankString("this is english")
	if err != nil {
		t.Fatalf("RankString: %v", err)
	}

	if len(ranks) != 3 {
		t.Fatalf("expected exactly 3 ranked languages, got %d", len(ranks))
	}

	classesSeen := make(map[string]bool)
	for _, r := range ranks {
		classesSeen[r.Language] = true
	}
	if !classesSeen["en"] || !classesSeen["es"] || !classesSeen["it"] {
		t.Errorf("ranks do not match expected subset: %v", ranks)
	}

	// First rank should be "en"
	if ranks[0].Language != "en" {
		t.Errorf("expected first rank to be en, got %q", ranks[0].Language)
	}
}

func TestIdentifyAndRankFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "english.txt")
	content := []byte("this is a simple english sentence to classify.")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write temporary file: %v", err)
	}

	// 1. Test package-level IdentifyFile
	res, err := IdentifyFile(filePath)
	if err != nil {
		t.Fatalf("IdentifyFile failed: %v", err)
	}
	if res.Language != "en" {
		t.Errorf("expected language 'en', got %q", res.Language)
	}

	// 2. Test package-level RankFile
	ranks, err := RankFile(filePath)
	if err != nil {
		t.Fatalf("RankFile failed: %v", err)
	}
	if len(ranks) == 0 {
		t.Fatalf("expected positive number of ranked languages, got 0")
	}
	if ranks[0].Language != "en" {
		t.Errorf("expected top rank language 'en', got %q", ranks[0].Language)
	}

	// 3. Test instance-method IdentifyFile on custom Identifier
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	res, err = id.IdentifyFile(filePath)
	if err != nil {
		t.Fatalf("id.IdentifyFile failed: %v", err)
	}
	if res.Language != "en" {
		t.Errorf("expected language 'en', got %q", res.Language)
	}

	// 4. Test instance-method RankFile on custom Identifier
	ranks, err = id.RankFile(filePath)
	if err != nil {
		t.Fatalf("id.RankFile failed: %v", err)
	}
	if len(ranks) == 0 {
		t.Fatalf("expected positive number of ranked languages, got 0")
	}
	if ranks[0].Language != "en" {
		t.Errorf("expected top rank language 'en', got %q", ranks[0].Language)
	}
}

func TestIdentifyAndRankFileErrors(t *testing.T) {
	// 1. Missing file path error check
	missingPath := "/nonexistent/path/to/some/file.txt"
	_, err := IdentifyFile(missingPath)
	if err == nil {
		t.Fatalf("expected IdentifyFile on missing file to fail, but it succeeded")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist, got %v", err)
	}

	_, err = RankFile(missingPath)
	if err == nil {
		t.Fatalf("expected RankFile on missing file to fail, but it succeeded")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist, got %v", err)
	}

	// Instance-level error check
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	_, err = id.IdentifyFile(missingPath)
	if err == nil {
		t.Fatalf("expected id.IdentifyFile on missing file to fail")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist, got %v", err)
	}

	_, err = id.RankFile(missingPath)
	if err == nil {
		t.Fatalf("expected id.RankFile on missing file to fail")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist, got %v", err)
	}
}

func TestConcurrentSetLanguagesAndIdentify(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	texts := [][]byte{
		[]byte("this is english"),
		[]byte("bonjour tout le monde"),
		[]byte("das ist ein deutscher satz"),
		[]byte("ciao a tutti"),
	}
	subsets := [][]string{
		{"en", "fr"},
		{"de", "it"},
		{"en", "es", "it"},
		nil,
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for j := range 500 {
				text := texts[j%len(texts)]
				res, err := id.IdentifyBytes(text)
				if err != nil {
					t.Errorf("IdentifyBytes failed: %v", err)
					return
				}
				if res.Language == "" {
					t.Errorf("IdentifyBytes returned empty language")
					return
				}

				ranks, err := id.RankBytes(text)
				if err != nil {
					t.Errorf("RankBytes failed: %v", err)
					return
				}
				if len(ranks) == 0 {
					t.Errorf("RankBytes returned no results")
					return
				}
			}
		})
	}

	for range 2 {
		wg.Go(func() {
			for j := range 200 {
				subset := subsets[j%len(subsets)]
				if subset == nil {
					id.ResetLanguages()
					continue
				}
				if err := id.SetLanguages(subset...); err != nil {
					t.Errorf("SetLanguages failed: %v", err)
					return
				}
			}
		})
	}

	wg.Wait()
}

func BenchmarkIdentifyBytesParallel(b *testing.B) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		b.Fatalf("failed to create default identifier: %v", err)
	}

	text := []byte("This is a moderately sized English sentence used to benchmark concurrent language identification.")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := id.IdentifyBytes(text); err != nil {
				b.Fatalf("IdentifyBytes failed: %v", err)
			}
		}
	})
}
