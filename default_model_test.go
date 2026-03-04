package langid

import "testing"

func TestClassifyWithEmbeddedDefaultModel(t *testing.T) {
	res, err := Classify("This is a short sentence in English.")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if res.Language == "" {
		t.Fatalf("empty language")
	}
}
