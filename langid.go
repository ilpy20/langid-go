package langid

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/ilpy20/langid-go/internal/modelio"
)

// Identifier classifies text by language using a pre-trained model.
type Identifier struct {
	model *modelio.Model
	pool  sync.Pool

	numLangs int
	classes  []string
	nbPC     []float32
	nbPTC    []float32
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
	return &Identifier{
		model:    m,
		numLangs: m.NumLangs,
		classes:  m.Classes,
		nbPC:     m.NbPC,
		nbPTC:    m.NbPTC,
	}
}

// Classify uses a lazily-initialized embedded default model.
func Classify(text string) (Result, error) {
	defaultOnce.Do(func() {
		defaultID, defaultErr = NewDefaultIdentifier()
	})
	if defaultErr != nil {
		return Result{}, defaultErr
	}
	return defaultID.IdentifyString(text)
}

// KeepOnly restricts the identifier to a specific subset of languages.
func (id *Identifier) KeepOnly(langs ...string) error {
	validLangs := make(map[string]bool)
	for _, l := range langs {
		validLangs[l] = true
	}

	var newClasses []string
	var newPC []float32
	keepIndices := make([]int, 0, len(langs))

	for i, c := range id.model.Classes {
		if validLangs[c] {
			keepIndices = append(keepIndices, i)
			newClasses = append(newClasses, c)
			newPC = append(newPC, id.model.NbPC[i])
		}
	}

	if len(newClasses) == 0 {
		return fmt.Errorf("none of the requested languages were found in the model")
	}

	newPTC := make([]float32, id.model.NumFeats*len(keepIndices))
	for feat := 0; feat < id.model.NumFeats; feat++ {
		baseOrig := feat * id.model.NumLangs
		baseNew := feat * len(keepIndices)
		for j, origIdx := range keepIndices {
			newPTC[baseNew+j] = id.model.NbPTC[baseOrig+origIdx]
		}
	}

	id.numLangs = len(keepIndices)
	id.classes = newClasses
	id.nbPC = newPC
	id.nbPTC = newPTC

	// Clear pool to recreate correctly sized buffers
	id.pool = sync.Pool{}

	return nil
}

// IdentifyString predicts a language label for text.
func (id *Identifier) IdentifyString(text string) (Result, error) {
	return id.IdentifyBytes([]byte(text))
}

// IdentifyBytes predicts a language label for bytes.
func (id *Identifier) IdentifyBytes(text []byte) (Result, error) {
	buf, err := id.getLogProbs(text)
	if err != nil {
		return Result{}, err
	}
	defer id.pool.Put(buf)

	best := 0
	for i := 1; i < id.numLangs; i++ {
		if buf.scores[i] > buf.scores[best] {
			best = i
		}
	}

	return Result{Language: id.classes[best], Score: buf.scores[best]}, nil
}

// RankString returns a sorted list of all languages and their raw log scores.
func (id *Identifier) RankString(text string) ([]Result, error) {
	return id.RankBytes([]byte(text))
}

// RankBytes returns a sorted list of all languages and their raw log scores.
func (id *Identifier) RankBytes(text []byte) ([]Result, error) {
	buf, err := id.getLogProbs(text)
	if err != nil {
		return nil, err
	}
	defer id.pool.Put(buf)

	results := make([]Result, id.numLangs)
	for i := 0; i < id.numLangs; i++ {
		results[i] = Result{
			Language: id.classes[i],
			Score:    buf.scores[i],
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
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

func (id *Identifier) getLogProbs(text []byte) (*workBuffer, error) {
	if id == nil || id.model == nil {
		return nil, fmt.Errorf("identifier is nil")
	}

	m := id.model
	buf, ok := id.pool.Get().(*workBuffer)
	if !ok {
		buf = newWorkBuffer(m.NumStates, m.NumFeats, id.numLangs)
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
		for i := uint16(0); i < n; i++ {
			feat := m.TkOutput[int(start)+int(i)]
			if buf.featCounts[feat] == 0 {
				buf.activeFeats = append(buf.activeFeats, feat)
			}
			buf.featCounts[feat] += count
		}
	}
	buf.activeStates = buf.activeStates[:0]

	for i := 0; i < id.numLangs; i++ {
		buf.scores[i] = float64(id.nbPC[i])
	}

	for _, feat := range buf.activeFeats {
		count := buf.featCounts[feat]
		buf.featCounts[feat] = 0

		base := int(feat) * id.numLangs
		c := float64(count)
		for lang := 0; lang < id.numLangs; lang++ {
			buf.scores[lang] += c * float64(id.nbPTC[base+lang])
		}
	}
	buf.activeFeats = buf.activeFeats[:0]

	return buf, nil
}
