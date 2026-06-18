package modelio

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestLoadRejectsInvalidModels(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*testModelWire)
		wantErrPart string
	}{
		{
			name: "zero languages",
			mutate: func(m *testModelWire) {
				m.numLangs = 0
				m.nbPC = nil
				m.nbPTC = nil
				m.classes = nil
			},
			wantErrPart: "num_langs must be positive",
		},
		{
			name: "transition out of range",
			mutate: func(m *testModelWire) {
				m.tkNextmove[0] = 2
			},
			wantErrPart: "transition 0 points to state 2",
		},
		{
			name: "output span past end",
			mutate: func(m *testModelWire) {
				m.tkOutputC[0] = 2
			},
			wantErrPart: "output span for state 0",
		},
		{
			name: "feature out of range",
			mutate: func(m *testModelWire) {
				m.tkOutput[0] = 1
			},
			wantErrPart: "output feature 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := newValidTestModelWire()
			tt.mutate(wire)

			_, err := loadFromReader(bytes.NewReader(wire.encode(t)))
			if err == nil {
				t.Fatal("expected loadFromReader to fail")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErrPart, err)
			}
		})
	}
}

type testModelWire struct {
	numFeats   uint32
	numLangs   uint32
	numStates  uint32
	tkNextmove []uint16
	tkOutputC  []uint16
	tkOutputS  []uint32
	tkOutput   []uint32
	nbPC       []float32
	nbPTC      []float32
	classes    []string
}

func newValidTestModelWire() *testModelWire {
	return &testModelWire{
		numFeats:   1,
		numLangs:   1,
		numStates:  1,
		tkNextmove: make([]uint16, 256),
		tkOutputC:  []uint16{1},
		tkOutputS:  []uint32{0},
		tkOutput:   []uint32{0},
		nbPC:       []float32{0},
		nbPTC:      []float32{0},
		classes:    []string{"en"},
	}
}

func (m *testModelWire) encode(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	write := func(v any) {
		t.Helper()
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatalf("binary write failed: %v", err)
		}
	}

	if _, err := buf.Write(magic[:]); err != nil {
		t.Fatalf("write magic failed: %v", err)
	}
	write(m.numFeats)
	write(m.numLangs)
	write(m.numStates)
	write(m.tkNextmove)
	write(m.tkOutputC)
	write(m.tkOutputS)
	write(uint32(len(m.tkOutput)))
	write(m.tkOutput)
	write(m.nbPC)
	write(m.nbPTC)

	for _, class := range m.classes {
		if len(class) > 0xffff {
			t.Fatalf("class label too long: %q", class)
		}
		write(uint16(len(class)))
		if _, err := buf.WriteString(class); err != nil {
			t.Fatalf("write class failed: %v", err)
		}
	}

	return buf.Bytes()
}

func TestLoadRejectsExceededLimits(t *testing.T) {
	tests := []struct {
		name        string
		feats       uint32
		langs       uint32
		states      uint32
		wantErrPart string
	}{
		{"num_feats too large", 1000001, 1, 1, "num_feats 1000001 exceeds limit"},
		{"num_langs too large", 1, 10001, 1, "num_langs 10001 exceeds limit"},
		{"num_states too large", 1, 1, 500001, "num_states 500001 exceeds limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.Write(magic[:])
			binary.Write(&buf, binary.LittleEndian, tt.feats)
			binary.Write(&buf, binary.LittleEndian, tt.langs)
			binary.Write(&buf, binary.LittleEndian, tt.states)

			_, err := loadFromReader(&buf)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErrPart, err)
			}
		})
	}

	t.Run("tk_output len too large", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write(magic[:])
		binary.Write(&buf, binary.LittleEndian, uint32(1)) // feats
		binary.Write(&buf, binary.LittleEndian, uint32(1)) // langs
		binary.Write(&buf, binary.LittleEndian, uint32(1)) // states

		// dummy payload for transitions (256 * 2), output counts (2), output offsets (4)
		buf.Write(make([]byte, 256*2+2+4))

		binary.Write(&buf, binary.LittleEndian, uint32(10000001)) // tkOutLen too large

		_, err := loadFromReader(&buf)
		if err == nil {
			t.Fatal("expected error")
		}
		expectedErr := "tk_output len 10000001 exceeds limit"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Fatalf("expected error containing %q, got %v", expectedErr, err)
		}
	})

	t.Run("nb_ptc len too large", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write(magic[:])
		binary.Write(&buf, binary.LittleEndian, uint32(1000000)) // feats
		binary.Write(&buf, binary.LittleEndian, uint32(100))     // langs
		binary.Write(&buf, binary.LittleEndian, uint32(1))       // states

		// dummy payload for transitions, output counts, output offsets
		buf.Write(make([]byte, 256*2+2+4))

		binary.Write(&buf, binary.LittleEndian, uint32(0)) // tkOutLen = 0 (so TkOutput is empty)

		// dummy payload for NbPC (100 float32s = 400 bytes)
		buf.Write(make([]byte, 100*4))

		_, err := loadFromReader(&buf)
		if err == nil {
			t.Fatal("expected error")
		}
		expectedErr := "nb_ptc len 100000000 exceeds limit"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Fatalf("expected error containing %q, got %v", expectedErr, err)
		}
	})

	t.Run("class label too long", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write(magic[:])
		binary.Write(&buf, binary.LittleEndian, uint32(1)) // feats
		binary.Write(&buf, binary.LittleEndian, uint32(1)) // langs
		binary.Write(&buf, binary.LittleEndian, uint32(1)) // states

		// dummy payload for transitions, output counts, output offsets
		buf.Write(make([]byte, 256*2+2+4))

		binary.Write(&buf, binary.LittleEndian, uint32(0)) // tkOutLen = 0 (so TkOutput is empty)

		// dummy payload for NbPC (1 float32 = 4 bytes)
		buf.Write(make([]byte, 4))
		// dummy payload for NbPTC (1 float32 = 4 bytes)
		buf.Write(make([]byte, 4))

		// write invalid class label length (e.g., 257)
		binary.Write(&buf, binary.LittleEndian, uint16(257))

		_, err := loadFromReader(&buf)
		if err == nil {
			t.Fatal("expected error")
		}
		expectedErr := "class[0] label length 257 exceeds limit 256"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Fatalf("expected error containing %q, got %v", expectedErr, err)
		}
	})
}
