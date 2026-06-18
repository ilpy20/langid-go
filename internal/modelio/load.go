package modelio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

var magic = [6]byte{'L', 'I', 'D', 'G', '1', 0}

const (
	maxNumLangs    = 10000
	maxNumFeats    = 1000000
	maxNumStates   = 500000
	maxTkOutputLen = 10000000
	maxNbPTCLen    = 50000000
)

func Load(path string) (*Model, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open model: %w", err)
	}
	defer f.Close()

	return loadFromReader(f)
}

// LoadBytes decodes a model from in-memory bytes.
func LoadBytes(data []byte) (*Model, error) {
	return loadFromReader(bytes.NewReader(data))
}

func loadFromReader(f io.Reader) (*Model, error) {
	m := &Model{}

	var gotMagic [6]byte
	if _, err := io.ReadFull(f, gotMagic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if gotMagic != magic {
		return nil, fmt.Errorf("invalid model magic")
	}

	var nFeats, nLangs, nStates uint32
	if err := binary.Read(f, binary.LittleEndian, &nFeats); err != nil {
		return nil, fmt.Errorf("read num_feats: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &nLangs); err != nil {
		return nil, fmt.Errorf("read num_langs: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &nStates); err != nil {
		return nil, fmt.Errorf("read num_states: %w", err)
	}

	var err error
	m.NumFeats, err = checkedUint32ToInt(nFeats, "num_feats")
	if err != nil {
		return nil, fmt.Errorf("invalid model dimensions: %w", err)
	}
	if m.NumFeats <= 0 {
		return nil, fmt.Errorf("invalid model: num_feats must be positive")
	}
	if m.NumFeats > maxNumFeats {
		return nil, fmt.Errorf("invalid model dimensions: num_feats %d exceeds limit %d", m.NumFeats, maxNumFeats)
	}

	m.NumLangs, err = checkedUint32ToInt(nLangs, "num_langs")
	if err != nil {
		return nil, fmt.Errorf("invalid model dimensions: %w", err)
	}
	if m.NumLangs <= 0 {
		return nil, fmt.Errorf("invalid model: num_langs must be positive")
	}
	if m.NumLangs > maxNumLangs {
		return nil, fmt.Errorf("invalid model dimensions: num_langs %d exceeds limit %d", m.NumLangs, maxNumLangs)
	}

	m.NumStates, err = checkedUint32ToInt(nStates, "num_states")
	if err != nil {
		return nil, fmt.Errorf("invalid model dimensions: %w", err)
	}
	if m.NumStates <= 0 {
		return nil, fmt.Errorf("invalid model: num_states must be positive")
	}
	if m.NumStates > maxNumStates {
		return nil, fmt.Errorf("invalid model dimensions: num_states %d exceeds limit %d", m.NumStates, maxNumStates)
	}

	nextLen, err := checkedProduct(m.NumStates, 256)
	if err != nil {
		return nil, fmt.Errorf("invalid model dimensions: %w", err)
	}
	m.TkNextmove = make([]uint16, nextLen)
	if err := binary.Read(f, binary.LittleEndian, &m.TkNextmove); err != nil {
		return nil, fmt.Errorf("read tk_nextmove: %w", err)
	}

	m.TkOutputC = make([]uint16, m.NumStates)
	if err := binary.Read(f, binary.LittleEndian, &m.TkOutputC); err != nil {
		return nil, fmt.Errorf("read tk_output_c: %w", err)
	}

	m.TkOutputS = make([]uint32, m.NumStates)
	if err := binary.Read(f, binary.LittleEndian, &m.TkOutputS); err != nil {
		return nil, fmt.Errorf("read tk_output_s: %w", err)
	}

	var tkOutLen uint32
	if err := binary.Read(f, binary.LittleEndian, &tkOutLen); err != nil {
		return nil, fmt.Errorf("read tk_output len: %w", err)
	}
	tkOutputLen, err := checkedUint32ToInt(tkOutLen, "tk_output len")
	if err != nil {
		return nil, fmt.Errorf("invalid model dimensions: %w", err)
	}
	if tkOutputLen < 0 || tkOutputLen > maxTkOutputLen {
		return nil, fmt.Errorf("invalid model dimensions: tk_output len %d exceeds limit %d", tkOutputLen, maxTkOutputLen)
	}
	m.TkOutput = make([]uint32, tkOutputLen)
	if err := binary.Read(f, binary.LittleEndian, &m.TkOutput); err != nil {
		return nil, fmt.Errorf("read tk_output: %w", err)
	}

	m.NbPC = make([]float32, m.NumLangs)
	if err := binary.Read(f, binary.LittleEndian, &m.NbPC); err != nil {
		return nil, fmt.Errorf("read nb_pc: %w", err)
	}

	nbPTCLen, err := checkedProduct(m.NumFeats, m.NumLangs)
	if err != nil {
		return nil, fmt.Errorf("invalid model dimensions: %w", err)
	}
	if nbPTCLen > maxNbPTCLen {
		return nil, fmt.Errorf("invalid model dimensions: nb_ptc len %d exceeds limit %d", nbPTCLen, maxNbPTCLen)
	}
	m.NbPTC = make([]float32, nbPTCLen)
	if err := binary.Read(f, binary.LittleEndian, &m.NbPTC); err != nil {
		return nil, fmt.Errorf("read nb_ptc: %w", err)
	}

	m.Classes = make([]string, m.NumLangs)
	for i := 0; i < m.NumLangs; i++ {
		var l uint16
		if err := binary.Read(f, binary.LittleEndian, &l); err != nil {
			return nil, fmt.Errorf("read class[%d] len: %w", i, err)
		}
		if l > 256 {
			return nil, fmt.Errorf("invalid model: class[%d] label length %d exceeds limit 256", i, l)
		}
		b := make([]byte, int(l))
		if _, err := io.ReadFull(f, b); err != nil {
			return nil, fmt.Errorf("read class[%d]: %w", i, err)
		}
		m.Classes[i] = string(b)
	}

	if err := validateModel(m); err != nil {
		return nil, err
	}

	return m, nil
}

func checkedProduct(a, b int) (int, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative dimension")
	}
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a > math.MaxInt/b {
		return 0, fmt.Errorf("dimension overflow: %d * %d", a, b)
	}
	return a * b, nil
}

func checkedUint32ToInt(v uint32, field string) (int, error) {
	if uint64(v) > uint64(math.MaxInt) {
		return 0, fmt.Errorf("%s overflows int: %d", field, v)
	}
	return int(v), nil
}

func validateModel(m *Model) error {
	if m.NumLangs <= 0 {
		return fmt.Errorf("invalid model: num_langs must be positive")
	}
	if m.NumFeats <= 0 {
		return fmt.Errorf("invalid model: num_feats must be positive")
	}
	if m.NumStates <= 0 {
		return fmt.Errorf("invalid model: num_states must be positive")
	}

	if len(m.Classes) != m.NumLangs {
		return fmt.Errorf("invalid model: expected %d classes, got %d", m.NumLangs, len(m.Classes))
	}
	if len(m.NbPC) != m.NumLangs {
		return fmt.Errorf("invalid model: expected %d class priors, got %d", m.NumLangs, len(m.NbPC))
	}

	expectedNextmove, err := checkedProduct(m.NumStates, 256)
	if err != nil {
		return fmt.Errorf("invalid model dimensions: %w", err)
	}
	if len(m.TkNextmove) != expectedNextmove {
		return fmt.Errorf("invalid model: expected %d transitions, got %d", expectedNextmove, len(m.TkNextmove))
	}

	expectedPTC, err := checkedProduct(m.NumFeats, m.NumLangs)
	if err != nil {
		return fmt.Errorf("invalid model dimensions: %w", err)
	}
	if len(m.NbPTC) != expectedPTC {
		return fmt.Errorf("invalid model: expected %d likelihoods, got %d", expectedPTC, len(m.NbPTC))
	}

	if len(m.TkOutputC) != m.NumStates {
		return fmt.Errorf("invalid model: expected %d output counts, got %d", m.NumStates, len(m.TkOutputC))
	}
	if len(m.TkOutputS) != m.NumStates {
		return fmt.Errorf("invalid model: expected %d output offsets, got %d", m.NumStates, len(m.TkOutputS))
	}

	for i, next := range m.TkNextmove {
		if int(next) >= m.NumStates {
			return fmt.Errorf("invalid model: transition %d points to state %d beyond %d", i, next, m.NumStates-1)
		}
	}

	for state := 0; state < m.NumStates; state++ {
		start, err := checkedUint32ToInt(m.TkOutputS[state], fmt.Sprintf("tk_output_s[%d]", state))
		if err != nil {
			return fmt.Errorf("invalid model dimensions: %w", err)
		}
		count := int(m.TkOutputC[state])
		end, err := checkedAdd(start, count)
		if err != nil {
			return fmt.Errorf("invalid model: output span for state %d overflows: %w", state, err)
		}
		if start < 0 || start > len(m.TkOutput) || end < start || end > len(m.TkOutput) {
			return fmt.Errorf("invalid model: output span for state %d is [%d:%d] with len %d", state, start, end, len(m.TkOutput))
		}
		for i := start; i < end; i++ {
			if int(m.TkOutput[i]) >= m.NumFeats {
				return fmt.Errorf("invalid model: output feature %d for state %d exceeds feature count %d", m.TkOutput[i], state, m.NumFeats)
			}
		}
	}

	return nil
}

func checkedAdd(a, b int) (int, error) {
	if b > 0 && a > math.MaxInt-b {
		return 0, fmt.Errorf("dimension overflow: %d + %d", a, b)
	}
	if b < 0 && a < math.MinInt-b {
		return 0, fmt.Errorf("dimension overflow: %d + %d", a, b)
	}
	return a + b, nil
}
