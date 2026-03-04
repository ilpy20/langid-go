package modelio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var magic = [6]byte{'L', 'I', 'D', 'G', '1', 0}

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

	m.NumFeats = int(nFeats)
	m.NumLangs = int(nLangs)
	m.NumStates = int(nStates)

	nextLen := int(nStates) * 256
	m.TkNextmove = make([]uint16, nextLen)
	if err := binary.Read(f, binary.LittleEndian, &m.TkNextmove); err != nil {
		return nil, fmt.Errorf("read tk_nextmove: %w", err)
	}

	m.TkOutputC = make([]uint16, int(nStates))
	if err := binary.Read(f, binary.LittleEndian, &m.TkOutputC); err != nil {
		return nil, fmt.Errorf("read tk_output_c: %w", err)
	}

	m.TkOutputS = make([]uint32, int(nStates))
	if err := binary.Read(f, binary.LittleEndian, &m.TkOutputS); err != nil {
		return nil, fmt.Errorf("read tk_output_s: %w", err)
	}

	var tkOutLen uint32
	if err := binary.Read(f, binary.LittleEndian, &tkOutLen); err != nil {
		return nil, fmt.Errorf("read tk_output len: %w", err)
	}
	m.TkOutput = make([]uint32, int(tkOutLen))
	if err := binary.Read(f, binary.LittleEndian, &m.TkOutput); err != nil {
		return nil, fmt.Errorf("read tk_output: %w", err)
	}

	m.NbPC = make([]float32, int(nLangs))
	if err := binary.Read(f, binary.LittleEndian, &m.NbPC); err != nil {
		return nil, fmt.Errorf("read nb_pc: %w", err)
	}

	nbPTCLen := int(nFeats) * int(nLangs)
	m.NbPTC = make([]float32, nbPTCLen)
	if err := binary.Read(f, binary.LittleEndian, &m.NbPTC); err != nil {
		return nil, fmt.Errorf("read nb_ptc: %w", err)
	}

	m.Classes = make([]string, int(nLangs))
	for i := 0; i < int(nLangs); i++ {
		var l uint16
		if err := binary.Read(f, binary.LittleEndian, &l); err != nil {
			return nil, fmt.Errorf("read class[%d] len: %w", i, err)
		}
		b := make([]byte, int(l))
		if _, err := io.ReadFull(f, b); err != nil {
			return nil, fmt.Errorf("read class[%d]: %w", i, err)
		}
		m.Classes[i] = string(b)
	}

	return m, nil
}
