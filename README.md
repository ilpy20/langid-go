# langid-go

**`langid-go`** is a high-performance Go port of the popular language identification tool [langid.py](https://github.com/saffsd/langid.py) and its C counterpart [langid.c](https://github.com/saffsd/langid.c). 

Like the originals, it comes pre-trained on 97 languages and is virtually insensitive to domain-specific features (e.g. HTML/XML markup). By leveraging Go's concurrency primitives (`sync.Pool`) and a flat-array "sparse set" architecture borrowed from `langid.c`, this port achieves **zero-allocation inference** on the hot loop, making it extremely fast and suitable for high-throughput stream processing.

## Features

- **Pre-trained on 97 languages** (ISO 639-1 codes).
- **Embedded Model:** Zero dependencies. The default model is compiled directly into the binary via `go:embed`.
- **Zero-Allocation Inference:** Highly optimized engine minimizes garbage collection overhead.
- **Language Subsetting:** Restrict predictions to a known subset of languages for improved accuracy and speed.
- **Probability Normalization:** Output standardized probabilities (0.0 - 1.0) rather than raw log-scores.
- **Versatile CLI:** Includes interactive, batch, and stream modes.

## Library Usage

```bash
go get github.com/ilpy20/langid-go@latest
```

### Basic Usage

The simplest way to classify text is by using the package-level `Classify` function, which automatically loads the embedded model on its first invocation.

```go
package main

import (
	"fmt"
	"github.com/ilpy20/langid-go"
)

func main() {
	res, err := langid.Classify("This is a short English sentence")
	if err == nil {
		// res.Language == "en", res.Score is the raw negative log probability
		fmt.Printf("Language: %s\n", res.Language)
	}
}
```

### Advanced Usage

To utilize advanced features like probability normalization, language ranking, or subsetting, you must instantiate an `Identifier`:

```go
package main

import (
	"fmt"
	"github.com/ilpy20/langid-go"
)

func main() {
	id, _ := langid.NewDefaultIdentifier()

	// 1. Restrict the language set 
	// This drastically improves accuracy and speed if your domain is known.
	id.KeepOnly("en", "fr", "es", "de", "it")

	// 2. Rank all languages instead of just returning the best match
	results, _ := id.RankString("Bonjour tout le monde")

	// 3. Normalize raw log-scores into a standard 0.0 - 1.0 probability distribution
	langid.Normalize(results)

	for i := 0; i < 3; i++ {
		fmt.Printf("%s: %.2f%%\n", results[i].Language, results[i].Score*100)
	}
	// Output: 
	// fr: 99.98%
	// en: 0.02%
	// es: 0.00%
}
```

## CLI Usage

`langid.go` provides a powerful command-line interface matching the functionality of the Python and C versions.

### Build

```bash
go build ./cmd/langid
```

### Options

```text
Usage of langid:
  -m, -model string
    	path to .lidg model (optional, uses default if omitted)
  -l, --langs string
    	comma-separated set of target ISO639 language codes (e.g en,de)
      --line
    	line mode: classify each input line (legacy alias: -l)
  -b, --batch
    	batch mode: treat stdin lines as file paths to classify
  -d, --dist
    	show full distribution over languages (rank mode)
  -n, --normalize
    	normalize confidence scores to probability values (0.0 to 1.0)
```

### Examples

**Interactive / Standard Mode:**
```bash
$ ./langid -n <<< "Hello World"
('en', 1.0000)
```

**Line Mode (`--line` or `-l`):** Process pipes line-by-line rather than as a single document.
```bash
$ printf "hello world\nbonjour tout le monde\n" | ./langid --line
('en', -102.5)
('fr', -105.1)
```

**Batch Mode (`-b` / `--batch`):** Treat lines as file paths, processing them in bulk. Useful with UNIX tools like `find`.
```bash
$ find . -name "*.md" | ./langid -b -n
./README.md,('en', 1.0000)
```

**Language Subsetting (`-l` / `--langs`):**
```bash
$ ./langid -n -l en,it <<< "Io non parlo italiano"
('it', 1.0000)
```

## Custom Models

The original `langid.py` tooling trains models and outputs them as Python 2 `pickle` files (e.g., `*.model`). 

To use custom models in `langid.go`, you must convert them to the highly-optimized `LIDG1` binary format using the provided Python script.

```bash
python3 scripts/convert_model.py custom.model model/custom.lidg
```
You can then load them in Go using `langid.LoadModel("model/custom.lidg")` or via the CLI with the `-m` flag.

## Acknowledgements and References

`langid.go` is a port of the Naive Bayes / DFA language identification algorithm originally created by Marco Lui and Timothy Baldwin. 

* [1] Lui, Marco and Timothy Baldwin (2011) *Cross-domain Feature Selection for Language Identification*, In Proceedings of the Fifth International Joint Conference on Natural Language Processing (IJCNLP 2011), Chiang Mai, Thailand, pp. 553—561. [PDF](http://www.aclweb.org/anthology/I11-1062)
* [2] Lui, Marco and Timothy Baldwin (2012) *langid.py: An Off-the-shelf Language Identification Tool*, In Proceedings of the 50th Annual Meeting of the Association for Computational Linguistics (ACL 2012), Demo Session, Jeju, Republic of Korea. [PDF](http://www.aclweb.org/anthology/P12-3005)
