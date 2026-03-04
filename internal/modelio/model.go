package modelio

// Model is a compact runtime representation of a langid model.
type Model struct {
	NumFeats  int
	NumLangs  int
	NumStates int

	TkNextmove []uint16
	TkOutputC  []uint16
	TkOutputS  []uint32
	TkOutput   []uint32

	NbPC    []float32
	NbPTC   []float32
	Classes []string
}
