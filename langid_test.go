package langid

import (
	"strings"
	"testing"
)

func TestSetLanguagesAndReset(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	origCount := id.numLangs
	if origCount == 0 {
		t.Fatalf("expected positive number of original languages, got 0")
	}

	// 1. Restrict to a valid subset ("en", "fr")
	err = id.SetLanguages("en", "fr")
	if err != nil {
		t.Fatalf("SetLanguages(\"en\", \"fr\") failed: %v", err)
	}

	if id.numLangs != 2 {
		t.Errorf("expected 2 active languages, got %d", id.numLangs)
	}

	if len(id.classes) != 2 {
		t.Errorf("expected 2 active classes, got %d", len(id.classes))
	}

	// Verify that the active classes are indeed "en" and "fr"
	classesMap := make(map[string]bool)
	for _, c := range id.classes {
		classesMap[c] = true
	}
	if !classesMap["en"] || !classesMap["fr"] {
		t.Errorf("active classes do not match [en, fr]: %v", id.classes)
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

	if id.numLangs != 2 {
		t.Errorf("expected 2 active languages, got %d", id.numLangs)
	}

	classesMap2 := make(map[string]bool)
	for _, c := range id.classes {
		classesMap2[c] = true
	}
	if !classesMap2["de"] || !classesMap2["it"] {
		t.Errorf("active classes do not match [de, it]: %v", id.classes)
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

	if id.numLangs != origCount {
		t.Errorf("expected restored count %d, got %d", origCount, id.numLangs)
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

	origCount := id.numLangs

	// 1. Partially invalid request
	err = id.SetLanguages("en", "invalid_lang_code", "fr")
	if err == nil {
		t.Fatalf("expected SetLanguages with partially invalid language to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), `"invalid_lang_code" is not supported`) {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify atomicity (state is unchanged)
	if id.numLangs != origCount {
		t.Errorf("expected identifier state to remain unchanged (count=%d) after partial failure, but got count=%d", origCount, id.numLangs)
	}

	// 2. Fully invalid request
	err = id.SetLanguages("invalid_one", "invalid_two")
	if err == nil {
		t.Fatalf("expected SetLanguages with fully invalid languages to fail, but it succeeded")
	}

	if id.numLangs != origCount {
		t.Errorf("expected identifier state to remain unchanged (count=%d) after full failure, but got count=%d", origCount, id.numLangs)
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
	if id.numLangs != origCount {
		t.Errorf("expected empty SetLanguages() to reset to full set (%d), got %d", origCount, id.numLangs)
	}
}

func TestKeepOnlyBackwardCompatibility(t *testing.T) {
	id, err := NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to create default identifier: %v", err)
	}

	origCount := id.numLangs

	// 1. KeepOnly should succeed on valid subset
	err = id.KeepOnly("en", "fr")
	if err != nil {
		t.Fatalf("KeepOnly failed: %v", err)
	}
	if id.numLangs != 2 {
		t.Errorf("expected 2 languages kept, got %d", id.numLangs)
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
	if id.numLangs != origCount {
		t.Errorf("expected reset to work, got %d vs %d", id.numLangs, origCount)
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
