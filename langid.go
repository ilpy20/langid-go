// Package langid provides a high-performance natural language identifier
// library supporting 97 languages. It is a pure Go runtime port of the langid
// inference stack, initially derived from langid.c and later expanded for
// parity with langid.js and langid.py.
//
// The package ports the Naive Bayes/DFA inference path, not the original
// training pipeline. It is CGO-free, making it simple to cross-compile and safe
// for highly concurrent production pipelines.
package langid

import (
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/ilpy20/langid-go/internal/modelio"
)

// Identifier classifies text by language using a pre-trained model.
type Identifier struct {
	model   *modelio.Model
	runtime atomic.Pointer[identifierRuntime]
}

// Result contains the best predicted class and its raw log score.
type Result struct {
	Language string
	Score    float64
}

type workBuffer struct {
	stateCounts  []uint32
	activeStates []uint16
	featCounts   []uint32
	activeFeats  []uint32
	scores       []float64
}

type identifierRuntime struct {
	numLangs int
	classes  []string
	nbPC     []float32
	nbPTC    []float32
	pool     *sync.Pool
}

func newWorkBuffer(numStates, numFeats, numLangs int) *workBuffer {
	return &workBuffer{
		stateCounts:  make([]uint32, numStates),
		activeStates: make([]uint16, 0, 256),
		featCounts:   make([]uint32, numFeats),
		activeFeats:  make([]uint32, 0, 256),
		scores:       make([]float64, numLangs),
	}
}

var (
	defaultOnce sync.Once
	defaultID   *Identifier
	defaultErr  error
)

// LoadModel reads a .lidg model file.
func LoadModel(path string) (*Identifier, error) {
	m, err := modelio.Load(path)
	if err != nil {
		return nil, err
	}
	return newIdentifier(m), nil
}

// NewDefaultIdentifier loads the embedded default model (ldpy).
func NewDefaultIdentifier() (*Identifier, error) {
	m, err := modelio.LoadBytes(defaultModel)
	if err != nil {
		return nil, err
	}
	return newIdentifier(m), nil
}

func newIdentifier(m *modelio.Model) *Identifier {
	id := &Identifier{model: m}
	id.runtime.Store(newIdentifierRuntime(m, m.Classes, m.NbPC, m.NbPTC))
	return id
}

func getDefaultIdentifier() (*Identifier, error) {
	defaultOnce.Do(func() {
		defaultID, defaultErr = NewDefaultIdentifier()
	})
	return defaultID, defaultErr
}

// Classify uses a lazily-initialized embedded default model.
func Classify(text string) (Result, error) {
	id, err := getDefaultIdentifier()
	if err != nil {
		return Result{}, err
	}
	return id.IdentifyString(text)
}

// IdentifyFile reads the file at the specified path and predicts its language using the default identifier.
func IdentifyFile(path string) (Result, error) {
	id, err := getDefaultIdentifier()
	if err != nil {
		return Result{}, err
	}
	return id.IdentifyFile(path)
}

// RankFile reads the file at the specified path and ranks all supported languages by likelihood using the default identifier.
func RankFile(path string) ([]Result, error) {
	id, err := getDefaultIdentifier()
	if err != nil {
		return nil, err
	}
	return id.RankFile(path)
}

// Classes returns the active language classes supported by the identifier.
func (id *Identifier) Classes() []string {
	rt := id.activeRuntime()
	if rt == nil {
		return nil
	}
	res := make([]string, len(rt.classes))
	copy(res, rt.classes)
	return res
}

// ResetLanguages restores the active language set of the identifier to include
// all languages present in the original loaded model.
func (id *Identifier) ResetLanguages() {
	if id == nil || id.model == nil {
		return
	}
	id.runtime.Store(newIdentifierRuntime(id.model, id.model.Classes, id.model.NbPC, id.model.NbPTC))
}

// SetLanguages restricts the active language set of the identifier to the specified subset.
// If langs is empty or nil, it resets the active languages to the original model languages.
// If any requested language is not supported by the model, it returns an error and leaves
// the active language set unmodified (atomic operation).
func (id *Identifier) SetLanguages(langs ...string) error {
	if id == nil || id.model == nil {
		return fmt.Errorf("identifier is nil")
	}
	if len(langs) == 0 {
		id.ResetLanguages()
		return nil
	}

	modelLangs := make(map[string]bool, len(id.model.Classes))
	for _, c := range id.model.Classes {
		modelLangs[c] = true
	}

	for _, l := range langs {
		if !modelLangs[l] {
			return fmt.Errorf("language %q is not supported by this model", l)
		}
	}

	validLangs := make(map[string]bool, len(langs))
	for _, l := range langs {
		validLangs[l] = true
	}

	var newClasses []string
	var newPC []float32
	keepIndices := make([]int, 0, len(validLangs))

	for i, c := range id.model.Classes {
		if validLangs[c] {
			keepIndices = append(keepIndices, i)
			newClasses = append(newClasses, c)
			newPC = append(newPC, id.model.NbPC[i])
		}
	}

	newPTC := make([]float32, id.model.NumFeats*len(keepIndices))
	for feat := 0; feat < id.model.NumFeats; feat++ {
		baseOrig := feat * id.model.NumLangs
		baseNew := feat * len(keepIndices)
		for j, origIdx := range keepIndices {
			newPTC[baseNew+j] = id.model.NbPTC[baseOrig+origIdx]
		}
	}

	id.runtime.Store(newIdentifierRuntime(id.model, newClasses, newPC, newPTC))
	return nil
}

// KeepOnly restricts the identifier to a specific subset of languages.
//
// Deprecated: Use SetLanguages instead, which has identical behavior with
// stricter language validation and support for resetting subsets.
func (id *Identifier) KeepOnly(langs ...string) error {
	if len(langs) == 0 {
		return fmt.Errorf("must specify at least one language to keep")
	}
	return id.SetLanguages(langs...)
}

// IdentifyString predicts a language label for text.
func (id *Identifier) IdentifyString(text string) (Result, error) {
	return id.IdentifyBytes([]byte(text))
}

// IdentifyBytes predicts a language label for bytes.
func (id *Identifier) IdentifyBytes(text []byte) (Result, error) {
	rt := id.activeRuntime()
	buf, err := id.getLogProbs(rt, text)
	if err != nil {
		return Result{}, err
	}
	defer rt.pool.Put(buf)

	best := 0
	for i := 1; i < rt.numLangs; i++ {
		if buf.scores[i] > buf.scores[best] {
			best = i
		}
	}

	return Result{Language: rt.classes[best], Score: buf.scores[best]}, nil
}

// RankString returns a sorted list of all languages and their raw log scores.
func (id *Identifier) RankString(text string) ([]Result, error) {
	return id.RankBytes([]byte(text))
}

// RankBytes returns a sorted list of all languages and their raw log scores.
func (id *Identifier) RankBytes(text []byte) ([]Result, error) {
	rt := id.activeRuntime()
	buf, err := id.getLogProbs(rt, text)
	if err != nil {
		return nil, err
	}
	defer rt.pool.Put(buf)

	results := make([]Result, rt.numLangs)
	for i := 0; i < rt.numLangs; i++ {
		results[i] = Result{
			Language: rt.classes[i],
			Score:    buf.scores[i],
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// IdentifyFile reads the file at the specified path and predicts its language.
// If reading the file fails, it returns the wrapped filesystem error without swallowing context.
func (id *Identifier) IdentifyFile(path string) (Result, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("failed to read file: %w", err)
	}
	return id.IdentifyBytes(content)
}

// RankFile reads the file at the specified path and ranks all supported languages by likelihood.
// If reading the file fails, it returns the wrapped filesystem error without swallowing context.
func (id *Identifier) RankFile(path string) ([]Result, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return id.RankBytes(content)
}

// Normalize converts a list of raw log-probabilities into a proper probability distribution (0.0 to 1.0).
func Normalize(results []Result) {
	if len(results) == 0 {
		return
	}

	maxScore := results[0].Score
	for _, r := range results {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	sum := 0.0
	for i := range results {
		results[i].Score = math.Exp(results[i].Score - maxScore)
		sum += results[i].Score
	}

	for i := range results {
		results[i].Score /= sum
	}
}

func (id *Identifier) getLogProbs(rt *identifierRuntime, text []byte) (*workBuffer, error) {
	if id == nil || id.model == nil || rt == nil {
		return nil, fmt.Errorf("identifier is nil")
	}

	m := id.model
	buf, ok := rt.pool.Get().(*workBuffer)
	if !ok {
		buf = newWorkBuffer(m.NumStates, m.NumFeats, rt.numLangs)
	}

	state := uint16(0)
	for _, b := range text {
		state = m.TkNextmove[int(state)*256+int(b)]
		if buf.stateCounts[state] == 0 {
			buf.activeStates = append(buf.activeStates, state)
		}
		buf.stateCounts[state]++
	}

	for _, st := range buf.activeStates {
		count := buf.stateCounts[st]
		buf.stateCounts[st] = 0

		s := int(st)
		start := m.TkOutputS[s]
		n := m.TkOutputC[s]
		for i := range n {
			feat := m.TkOutput[int(start)+int(i)]
			if buf.featCounts[feat] == 0 {
				buf.activeFeats = append(buf.activeFeats, feat)
			}
			buf.featCounts[feat] += count
		}
	}
	buf.activeStates = buf.activeStates[:0]

	for i := 0; i < rt.numLangs; i++ {
		buf.scores[i] = float64(rt.nbPC[i])
	}

	for _, feat := range buf.activeFeats {
		count := buf.featCounts[feat]
		buf.featCounts[feat] = 0

		base := int(feat) * rt.numLangs
		c := float64(count)
		for lang := 0; lang < rt.numLangs; lang++ {
			buf.scores[lang] += c * float64(rt.nbPTC[base+lang])
		}
	}
	buf.activeFeats = buf.activeFeats[:0]

	return buf, nil
}

func (id *Identifier) activeRuntime() *identifierRuntime {
	if id == nil {
		return nil
	}
	return id.runtime.Load()
}

func newIdentifierRuntime(m *modelio.Model, classes []string, nbPC []float32, nbPTC []float32) *identifierRuntime {
	rt := &identifierRuntime{
		numLangs: len(classes),
		classes:  classes,
		nbPC:     nbPC,
		nbPTC:    nbPTC,
	}
	rt.pool = &sync.Pool{
		New: func() any {
			return newWorkBuffer(m.NumStates, m.NumFeats, rt.numLangs)
		},
	}
	return rt
}
