package langid

import (
	_ "embed"
)

//go:embed model/ldpy.lidg
var defaultModel []byte
