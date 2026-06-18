package langid

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type pythonPrediction struct {
	Text  string  `json:"text"`
	Lang  string  `json:"lang"`
	Score float64 `json:"score"`
}

type pythonReferenceResults struct {
	Classes     []string           `json:"classes"`
	NumLangs    int                `json:"num_langs"`
	NumFeats    int                `json:"num_feats"`
	NumStates   int                `json:"num_states"`
	Predictions []pythonPrediction `json:"predictions"`
}

func TestParityWithLegacyModelReference(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	root := moduleRoot(t)
	models := []struct {
		name string
		path string
	}{
		{
			name: "ldpy",
			path: filepath.Join(root, "..", "langid.c", "ldpy.model"),
		},
		{
			name: "acquis",
			path: filepath.Join(root, "..", "langid.c", "acquis.model"),
		},
	}

	for _, tc := range models {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); err != nil {
				t.Fatalf("missing legacy model %q: %v", tc.path, err)
			}

			tmp := t.TempDir()
			converted := filepath.Join(tmp, tc.name+".lidg")

			// Convert legacy model dynamically
			convert := exec.Command("python3", filepath.Join(root, "scripts", "convert_model.py"), tc.path, converted)
			convert.Dir = root
			if out, err := convert.CombinedOutput(); err != nil {
				t.Fatalf("convert model: %v\n%s", err, out)
			}

			// Load the converted Go model
			id, err := LoadModel(converted)
			if err != nil {
				t.Fatalf("load converted model: %v", err)
			}

			samples := []string{
				"hello world",
				"this is a short English sentence",
				"bonjour tout le monde",
				"ceci est une phrase en francais",
				"hola amigos",
				"esta es una frase en espanol",
				"ciao a tutti",
				"questa e una frase in italiano",
				"hallo welt",
				"das ist ein deutscher satz",
				"ola mundo",
				"isto e uma frase em portugues",
				"hej varlden",
				"detta ar en svensk mening",
				"dit is een zin in het nederlands",
				"to jest zdanie po polsku",
				"privet mir",
				"merhaba dunya",
				"xin chao tat ca moi nguoi",
				"watashi wa gengo nintei no tesuto desu",
			}

			// Fetch reference predictions and metadata from Python
			pyRef := getPythonReferenceResults(t, tc.path, samples)

			// 1. Verify expected metadata and dimensions
			if id.model.NumLangs != pyRef.NumLangs {
				t.Errorf("metadata mismatch NumLangs: go=%d python=%d", id.model.NumLangs, pyRef.NumLangs)
			}
			if id.model.NumFeats != pyRef.NumFeats {
				t.Errorf("metadata mismatch NumFeats: go=%d python=%d", id.model.NumFeats, pyRef.NumFeats)
			}
			if id.model.NumStates != pyRef.NumStates {
				t.Errorf("metadata mismatch NumStates: go=%d python=%d", id.model.NumStates, pyRef.NumStates)
			}

			// 2. Verify language classes list parity (names, length, and ordering)
			if len(id.Classes()) != len(pyRef.Classes) {
				t.Fatalf("classes list length mismatch: go=%d python=%d", len(id.Classes()), len(pyRef.Classes))
			}
			for i, c := range id.Classes() {
				if c != pyRef.Classes[i] {
					t.Errorf("classes list mismatch at index %d: go=%q python=%q", i, c, pyRef.Classes[i])
				}
			}

			// 3. Verify prediction and score parity
			for i, s := range samples {
				res, err := id.IdentifyString(s)
				if err != nil {
					t.Fatalf("go classify sample %d (%q): %v", i, s, err)
				}

				pyPred := pyRef.Predictions[i]
				if res.Language != pyPred.Lang {
					t.Errorf("prediction mismatch sample %d (%q): go=%q py=%q", i, s, res.Language, pyPred.Lang)
				}

				// Check score within epsilon of 1e-4 (accounting for summation order differences between float32/float64)
				diff := math.Abs(res.Score - pyPred.Score)
				if diff > 1e-4 {
					t.Errorf("score mismatch sample %d (%q): go=%f py=%f (diff=%f)", i, s, res.Score, pyPred.Score, diff)
				}
			}
		})
	}
}

func getPythonReferenceResults(t *testing.T, legacyModel string, samples []string) pythonReferenceResults {
	t.Helper()

	script := `
import sys
import json
import array
import base64
import bz2
import collections
import io
import pickle

def compat_array(typecode, initializer=None):
    if isinstance(typecode, bytes):
        typecode = typecode.decode("latin1")
    out = array.array(typecode)
    if initializer is None:
        return out
    if isinstance(initializer, str):
        initializer = initializer.encode("latin1")
    if isinstance(initializer, (bytes, bytearray)):
        out.frombytes(initializer)
    else:
        out.extend(initializer)
    return out

class CompatUnpickler(pickle.Unpickler):
    def find_class(self, module, name):
        if module == "array" and name == "array":
            return compat_array
        return super().find_class(module, name)

def load_model(path):
    raw = open(path, "rb").read()
    payload = bz2.decompress(base64.b64decode(raw))
    nb_ptc, nb_pc, nb_classes, tk_nextmove, tk_output = CompatUnpickler(io.BytesIO(payload), encoding="latin1").load()
    classes = []
    for c in nb_classes:
        if isinstance(c, bytes):
            classes.append(c.decode("utf-8"))
        else:
            classes.append(str(c))
    num_langs = len(nb_pc)
    num_feats = len(nb_ptc) // num_langs
    num_states = len(tk_nextmove) // 256
    return nb_ptc, nb_pc, classes, tk_nextmove, tk_output, num_langs, num_feats, num_states

def classify(text, nb_ptc, nb_pc, classes, tk_nextmove, tk_output):
    state_counts = collections.defaultdict(int)
    state = 0
    for b in text.encode("utf-8"):
        state = tk_nextmove[(state << 8) + b]
        state_counts[state] += 1

    feat_counts = collections.defaultdict(int)
    for s, c in state_counts.items():
        for f in tk_output.get(s, []):
            feat_counts[f] += c

    num_langs = len(nb_pc)
    scores = [float(x) for x in nb_pc]
    for feat, c in feat_counts.items():
        base = feat * num_langs
        for j in range(num_langs):
            scores[j] += c * float(nb_ptc[base + j])

    best = max(range(num_langs), key=lambda i: scores[i])
    return classes[best], scores[best]

def main():
    if len(sys.argv) < 2:
        print("missing model path", file=sys.stderr)
        sys.exit(1)
    model_path = sys.argv[1]
    nb_ptc, nb_pc, classes, tk_nextmove, tk_output, num_langs, num_feats, num_states = load_model(model_path)
    
    samples = json.load(sys.stdin)
    predictions = []
    for s in samples:
        lang, score = classify(s, nb_ptc, nb_pc, classes, tk_nextmove, tk_output)
        predictions.append({
            "text": s,
            "lang": lang,
            "score": score
        })
    
    out = {
        "classes": classes,
        "num_langs": num_langs,
        "num_feats": num_feats,
        "num_states": num_states,
        "predictions": predictions
    }
    print(json.dumps(out))

if __name__ == "__main__":
    main()
`

	samplesJSON, err := json.Marshal(samples)
	if err != nil {
		t.Fatalf("marshal samples: %v", err)
	}

	cmd := exec.Command("python3", "-c", script, legacyModel)
	cmd.Stdin = bytes.NewReader(samplesJSON)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("python reference process failed: %v\nstderr:\n%s", err, stderr.String())
	}

	var res pythonReferenceResults
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal python results: %v\nstdout:\n%s", err, stdout.String())
	}

	return res
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
