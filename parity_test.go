package langid

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParityWithLegacyModelReference(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	root := moduleRoot(t)
	legacyModel := filepath.Join(root, "..", "langid.c", "ldpy.model")
	if _, err := os.Stat(legacyModel); err != nil {
		t.Fatalf("missing legacy model %q: %v", legacyModel, err)
	}

	tmp := t.TempDir()
	converted := filepath.Join(tmp, "ldpy.lidg")

	convert := exec.Command("python3", filepath.Join(root, "scripts", "convert_model.py"), legacyModel, converted)
	convert.Dir = root
	if out, err := convert.CombinedOutput(); err != nil {
		t.Fatalf("convert model: %v\n%s", err, out)
	}

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

	goPreds := make([]string, len(samples))
	for i, s := range samples {
		res, err := id.IdentifyString(s)
		if err != nil {
			t.Fatalf("go classify sample %d: %v", i, err)
		}
		goPreds[i] = res.Language
	}

	pyPreds := classifyWithPythonReference(t, legacyModel, samples)
	if len(pyPreds) != len(goPreds) {
		t.Fatalf("python predictions mismatch: got=%d want=%d", len(pyPreds), len(goPreds))
	}

	for i := range goPreds {
		if goPreds[i] != pyPreds[i] {
			t.Fatalf("parity mismatch sample %d (%q): go=%q py=%q", i, samples[i], goPreds[i], pyPreds[i])
		}
	}
}

func classifyWithPythonReference(t *testing.T, legacyModel string, samples []string) []string {
	t.Helper()

	script := `
import argparse
import array
import base64
import bz2
import collections
import io
import pickle
import sys

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
    return nb_ptc, nb_pc, classes, tk_nextmove, tk_output

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
    return classes[best]

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("model")
    args = ap.parse_args()
    nb_ptc, nb_pc, classes, tk_nextmove, tk_output = load_model(args.model)

    for line in sys.stdin:
        line = line.rstrip("\n")
        print(classify(line, nb_ptc, nb_pc, classes, tk_nextmove, tk_output))

if __name__ == "__main__":
    main()
`

	cmd := exec.Command("python3", "-c", script, legacyModel)
	cmd.Stdin = strings.NewReader(strings.Join(samples, "\n") + "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("python reference classify: %v\nstderr:\n%s", err, stderr.String())
	}

	var out []string
	s := bufio.NewScanner(&stdout)
	for s.Scan() {
		out = append(out, strings.TrimSpace(s.Text()))
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan python output: %v", err)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
