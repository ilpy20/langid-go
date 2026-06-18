# langid-go

**`langid-go`** is a high-performance Go port of the popular language identification tool [langid.py](https://github.com/saffsd/langid.py) and its C counterpart [langid.c](https://github.com/saffsd/langid.c). 

Like the originals, it comes pre-trained on 97 languages and is virtually insensitive to domain-specific features (e.g. HTML/XML markup). By leveraging Go's concurrency primitives (`sync.Pool`) and a flat-array "sparse set" architecture borrowed from `langid.c`, this port achieves **zero-allocation inference** on the hot loop, making it extremely fast and suitable for high-throughput stream processing.

## Background & Motivation

In building production-grade language classification pipelines in Go, developers face significant gaps in the native NLP ecosystem:
- **Limitations of `lingua-go`**: While feature-rich, the popular `lingua-go` library has been largely abandoned for years. Operationally, it suffers from severe bugs when processing short texts, where it frequently misclassifies inputs and returns random or incorrect languages. It also introduces high computational and memory overhead.
- **Limitations of `whatlanggo`**: While extremely fast, `whatlanggo` supports a limited set of languages and formats its output exclusively in ISO 639-3 (three-letter codes), which is incompatible with downstream pipelines that require standard ISO 639-1 two-letter codes.
- **Fragility of CGO Wrappers**: Previous attempts to bring the proven, robust Naive Bayes algorithm of `langid` to Go relied entirely on fragile CGO bindings (such as [dbalan/langid_go](https://github.com/dbalan/langid_go) wrapping `langid.c`). CGO introduces severe runtime thread overhead, interferes with Go's garbage collector and memory tracking, and complicates cross-compilation.

**`langid-go`** solves these issues by offering a **pure, 100% Go implementation** that achieves exact mathematical parity with the original Python unpickler and Naive Bayes vector engine, while running with **zero allocations** in standard concurrency-safe hot paths.

## Features

- **Pre-trained on 97 languages** (ISO 639-1 codes).
- **Embedded Model:** Zero dependencies. The default model is compiled directly into the binary via `go:embed`.
- **Zero-Allocation Inference:** Highly optimized engine minimizes garbage collection overhead.
- **Language Subsetting:** Restrict predictions to a known subset of languages for improved accuracy and speed.
- **Probability Normalization:** Output standardized probabilities (0.0 - 1.0) rather than raw log-scores.
- **Versatile CLI:** Includes interactive, batch, and stream modes.
- **CGO-Free:** 100% pure Go, guaranteeing simple cross-compilation and native GC performance.

## Compatibility Matrix

`langid-go` has been designed as a fully-featured, production-ready superset of the original ecosystem libraries:

| Feature | langid.py | langid.c | langid.js | langid-go |
| --- | --- | --- | --- | --- |
| **Default 97-language model** | yes | yes | yes | **yes** |
| **Custom model loading** | yes | yes | via generated JS | **yes (via `.lidg` binary)** |
| **Classify text** | yes | yes | yes | **yes** |
| **Return raw log-score** | yes | no | yes (as ranks) | **yes** |
| **Rank all languages** | yes | no | yes | **yes** |
| **Normalize probabilities** | yes | no | no | **yes** |
| **Language subsetting** | yes | no | no | **yes** |
| **Reset language subset** | yes | no | no | **yes** |
| **File helper API** | yes | CLI only | no | **yes** |
| **CLI document mode** | yes | yes | no | **yes** |
| **CLI line mode** | yes | yes | no | **yes** |
| **CLI batch mode** | yes | yes | no | **yes** |
| **Python-compatible flags** | yes | partial | no | **yes** |
| **URL classification** | yes | no | no | **yes** |
| **HTTP service mode** | yes | no | no | **yes** |
| **Web browser demo** | yes | no | yes | **yes** |
| **Training tools** | yes | no | no | *Planned Future Feature (TODO)* |

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

## Model Training & Customization

### Scope and Decisions
`langid-go` is designed as a **high-speed, highly concurrent, zero-allocation inference engine**. To keep the Go package optimized, secure, and free from external runtime dependencies or floating-point precision drift, the following architectural choices have been made:
- **Go-Native Training is a Planned Future Feature (TODO)**: Model training requires a multi-stage statistical pipeline (corpus indexing, byte-level sliding window tokenization, Shannon information-gain calculations, n-gram optimization, and Aho-Corasick DFA state-machine construction). The reference `langid.py` implementation utilizes the Python scientific stack (`numpy` and `scipy`) for these calculations. Implementing a native Go training pipeline remains a planned future feature (TODO) once suitable Go NLP, matrix computation, or scanner compiling libraries are identified.
- **Direct Legacy Model Loading (`.model` files) is Out of Scope**: Original models produced by Python are base64-encoded, bz2-compressed Python 2 `pickle` files. Reading Python pickles directly in Go is fragile, insecure, and computationally expensive.

Instead, custom models are trained using the reference Python pipeline and converted to the highly-optimized, type-safe Go `.lidg` binary format. The provided conversion utility (`scripts/convert_model.py`) is modeled directly on and adapted from the original [`ldpy2ldc.py`](https://github.com/saffsd/langid.c/blob/master/ldpy2ldc.py) script in the `langid.c` package.

### Custom Model Workflow

#### 1. Train Your Model in Python
Use the official Python training toolkit located in the reference repository under [langid.py/langid/train](https://github.com/saffsd/langid.py/tree/master/langid/train). 
To train a model from a corpus directory (where each subdirectory corresponds to a language code containing text documents), run:
```bash
python3 path/to/langid.py/langid/train/train.py -m /path/to/output_dir /path/to/corpus
```
This produces a legacy Python model file (e.g., `my_custom.model`).

#### 2. Convert Your Model to Go `.lidg` Format
Convert the legacy pickle model into the highly-optimized `.lidg` binary format using the provided conversion utility:
```bash
python3 scripts/convert_model.py my_custom.model model/my_custom.lidg
```

#### 3. Load Your Custom Model in Go
You can load and run your custom `.lidg` model programmatically or via the command line.

**Programmatically:**
```go
package main

import (
	"fmt"
	"github.com/ilpy20/langid-go"
)

func main() {
	id, err := langid.LoadModel("model/my_custom.lidg")
	if err != nil {
		panic(err)
	}

	res, _ := id.IdentifyString("This text will be classified by your custom model")
	fmt.Printf("Language: %s (Log Score: %.2f)\n", res.Language, res.Score)
}
```

**Via the CLI:**
Specify your custom model using the `-m` or `--model` flag:
```bash
./langid -m model/my_custom.lidg <<< "This text will be classified by your custom model"
```

## Acknowledgements and References

`langid.go` is a port of the Naive Bayes / DFA language identification algorithm originally created by Marco Lui and Timothy Baldwin. 

* [1] Lui, Marco and Timothy Baldwin (2011) *Cross-domain Feature Selection for Language Identification*, In Proceedings of the Fifth International Joint Conference on Natural Language Processing (IJCNLP 2011), Chiang Mai, Thailand, pp. 553—561. [PDF](http://www.aclweb.org/anthology/I11-1062)
* [2] Lui, Marco and Timothy Baldwin (2012) *langid.py: An Off-the-shelf Language Identification Tool*, In Proceedings of the 50th Annual Meeting of the Association for Computational Linguistics (ACL 2012), Demo Session, Jeju, Republic of Korea. [PDF](http://www.aclweb.org/anthology/P12-3005)
