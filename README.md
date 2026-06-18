[![Go Reference](https://pkg.go.dev/badge/github.com/ilpy20/langid-go.svg)](https://pkg.go.dev/github.com/ilpy20/langid-go)

# langid-go

**`langid-go`** is a high-performance Go port of the popular language identification tool [langid.py](https://github.com/saffsd/langid.py) and its C counterpart [langid.c](https://github.com/saffsd/langid.c). 

Like the originals, it comes pre-trained on 97 languages and is virtually insensitive to domain-specific features (e.g. HTML/XML markup). By leveraging Go's concurrency primitives (`sync.Pool`) and a flat-array "sparse set" architecture borrowed from `langid.c`, this port achieves **zero-allocation inference** on the hot loop, making it extremely fast and suitable for high-throughput stream processing.

## Background & Motivation

In building production-grade language classification pipelines in Go, developers face significant gaps in the native NLP ecosystem:

- **The Fall of `lingua-go`**: When [`lingua-go`](https://github.com/pemistahl/lingua-go) was originally released, it was marketed as the "most accurate language detector for Go," specifically claiming superior accuracy on short sentences compared to lightweight alternatives. However, the Go port has been effectively abandoned (while its Rust counterpart [`lingua-rs`](https://github.com/pemistahl/lingua-rs) continues to be actively maintained, leaving the Go project stuck in a perpetual state of transition, see [lingua-go#68](https://github.com/pemistahl/lingua-go/issues/68)). Operationally, it suffers from critical issues:
  * **Short & Mixed Text Fragility**: In practice, `lingua-go` frequently fails on short sentences containing mixed scripts, multilingual vocabularies, or modern brand/product names, often throwing incorrect, entirely random language predictions (see [lingua-go#82](https://github.com/pemistahl/lingua-go/issues/82)).
  * **Severe Repository Bloat**: The module is incredibly heavy because it is bloated with unnecessary test data files rather than maintaining just the necessary runtime models or active clean training pipelines (see [lingua-go#78](https://github.com/pemistahl/lingua-go/issues/78)).
  * **Resource Overhead**: It introduces prohibitive computational and memory allocations on high-throughput pipelines.

- **The Abandonment of `whatlanggo`**: While extremely fast, [`whatlanggo`](https://github.com/abadojack/whatlanggo) was abandoned over 7 years ago (outstanding pull requests like [whatlanggo#22](https://github.com/abadojack/whatlanggo/pull/22) and [whatlanggo#27](https://github.com/abadojack/whatlanggo/pull/27) remain unmerged, despite its Rust predecessor [`whatlang-rs`](https://github.com/greyblake/whatlang-rs) remaining active). It exhibits severe limitations:
  * **Short Text Failures**: Like `lingua-go`, `whatlanggo` struggles with short or single-word inputs, often failing to detect languages or misidentifying them completely (see [whatlanggo#21](https://github.com/abadojack/whatlanggo/issues/21)).
  * **Incompatible Format**: It returns results exclusively in three-letter ISO 639-3 codes, requiring custom mapping wrappers to integrate with pipelines utilizing standard ISO 639-1 two-letter codes.
  * **Limited Language Support**: It supports a restricted set of languages compared to modern NLP needs.

- **Fragility of CGO Wrappers**: Previous attempts to bring the proven, robust Naive Bayes algorithm of `langid` to Go relied entirely on fragile CGO bindings (such as [dbalan/langid_go](https://github.com/dbalan/langid_go) wrapping `langid.c`). CGO introduces severe runtime thread overhead, interferes with Go's garbage collector and memory tracking, and complicates cross-compilation.

**`langid-go`** solves these issues by offering a **pure, 100% Go implementation** that achieves exact mathematical parity with the original Python unpickler and Naive Bayes vector engine, while running with **zero allocations** in standard concurrency-safe hot paths.

### Why this architecture is critical for modern LLM Chatbots and AI Agents:
- **Robustness to Messy, Short Inputs**: LLM chatbot prompts and agent instructions are notoriously short, fragmented, and noisy. They are heavily polluted with brand names, technical jargon, code syntax, mixed scripts, and emojis. Because `langid` utilizes a Naive Bayes classifier trained via cross-domain Information Gain (IG) feature selection, it remains virtually immune to these domain-specific artifacts, avoiding the random misclassifications that plague traditional character-distance models.
- **Embedded & Zero-Allocation**: Running language classification via external microservices or heavy Python dependencies introduces unacceptable latency into real-time chatbot routing. `langid-go` embeds the model directly via `go:embed` and performs inference with zero garbage collection overhead on the hot path, making it perfect for high-throughput LLM gateway routing.
- **Model Portability & Future Native Training**: While other libraries bundle black-box models or bloat repositories with static test data, `langid-go` provides a clean pipeline for loading custom `.lidg` binary models. In addition, its [roadmap](./TODO.md) includes native Go-native training, enabling developers to build and adapt language classification models dynamically within their Go agent runtimes.

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
	// Use SetLanguages(...) instead of the deprecated KeepOnly(...)
	id.SetLanguages("en", "fr", "es", "de", "it")

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

> [!NOTE]
> You can restore the full language list at any time by calling `id.ResetLanguages()` or invoking `id.SetLanguages()` with empty/no arguments.

### File Helper APIs

`langid-go` provides optimized native file-reading classification helpers at both the package and instance levels, ensuring clean error propagation:

```go
// Package-level helpers (uses default embedded model)
res, err := langid.IdentifyFile("document.txt")
results, err := langid.RankFile("document.txt")

// Instance-level helpers
res, err := id.IdentifyFile("document.txt")
results, err := id.RankFile("document.txt")
```

### URL Classification API

The `urlclass` package provides a programmatic client to fetch and classify the text contents of standard web pages with automatic timeout management:

```go
package main

import (
	"fmt"
	"time"

	"github.com/ilpy20/langid-go"
	"github.com/ilpy20/langid-go/urlclass"
)

func main() {
	id, _ := langid.NewDefaultIdentifier()
	client := urlclass.NewClient(id)

	// Fetch a URL and classify its language with a 5-second timeout
	res, bytesFetched, err := client.ClassifyURL("https://example.com", 5*time.Second)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Fetched %d bytes. Language: %s (Log Score: %.2f)\n", bytesFetched, res.Language, res.Score)
}
```

### HTTP Service API

The `service` package provides a highly-concurrent HTTP router and server wrapping `langid` for exposing classification over REST endpoints or hosting an interactive local sandbox:

```go
package main

import (
	"github.com/ilpy20/langid-go"
	"github.com/ilpy20/langid-go/service"
)

func main() {
	id, _ := langid.NewDefaultIdentifier()
	srv := service.NewServer(id)

	// Starts an HTTP server on http://localhost:9008
	if err := srv.Start("127.0.0.1", 9008); err != nil {
		panic(err)
	}
}
```

---

## CLI Usage

`langid` provides a powerful pure-Go command-line interface fully backwards-compatible with the original Python and C versions, while introducing advanced modern tooling.

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
  -f, --format string
    	output format for batch mode: classic, csv, or jsonl (default "classic")
      --ignore-missing
    	silently skip missing or unreadable files in batch mode
      --serve
    	start HTTP service mode
      --demo
    	start HTTP service mode and open demo page in web browser
      --host string
    	host to bind HTTP service to (default "127.0.0.1")
      --port int
    	port to bind HTTP service to (default 9008)
  -u, --url string
    	classify the content of a URL
```

### Modes of Operation

#### 1. Standard / Pipe Mode
Process a single document from standard input.
```bash
$ ./langid -n <<< "Hello World"
('en', 1.0000)
```

#### 2. Interactive REPL Mode
Running `./langid` with standard input attached to a terminal automatically boots an interactive shell.
```bash
$ ./langid -n
>>> Hello World
('en', 1.0000)
>>> Bonjour tout le monde
('fr', 1.0000)
```

#### 3. Line Mode (`--line`)
Process standard input line-by-line, treating each as a distinct classification job.
```bash
$ printf "hello world\nbonjour tout le monde\n" | ./langid --line
('en', -102.5)
('fr', -105.1)
```

#### 4. Batch Mode (`-b` / `--batch` / `-f` / `--format`)
Treat inputs as file paths, processing them in bulk. Files can be passed directly as command-line arguments, falling back to reading paths from stdin if none are specified.

##### Output Formats (`--format classic | csv | jsonl`)
* **`classic` (default)**: Prints `path,('lang', score)`
* **`csv`**:
  * In standard mode, outputs: `path,lang,score` (no header)
  * In distribution mode (`-d`), outputs a header row followed by scores for all supported columns:
    ```text
    path,en,fr,es,de,...
    file1.txt,-105.1,-240.2,...
    ```
* **`jsonl`**:
  * In standard mode:
    ```json
    {"path":"file1.txt","language":"en","confidence":-105.1}
    ```
  * In distribution mode (`-d`):
    ```json
    {"path":"file1.txt","ranking":[{"language":"en","score":-105.1},{"language":"fr","score":-240.2},...]}
    ```

##### Command Examples:
```bash
# Pass files directly as arguments
./langid --batch --format csv file1.txt file2.txt

# Pipe file lists from Unix utilities
find . -name "*.md" | ./langid -b -n --format jsonl

# Ignore missing/unreadable files (instead of returning "NOSUCHFILE")
./langid --batch --ignore-missing file1.txt missing_file.txt
```

#### 5. URL Classification (`-u` / `--url`)
Directly retrieve and classify webpage contents from the command line:
```bash
$ ./langid --url "https://example.com" -n
https://example.com 1256 ('en', 1.0000)
```
*(Outputs the target URL, the response body length in bytes, and the predicted language metadata).*

#### 6. HTTP Server & Web Demo Mode (`--serve` / `--demo`)
Expose language identification as an HTTP microservice:
```bash
# Starts service locally
./langid --serve --port 9008

# Starts service and opens the interactive jQuery sandbox demo in your default browser
./langid --demo
```

##### API Specifications:
* **`POST /detect`** or **`GET /detect?q=<text>`**: Predict the language.
  * Request Body: Raw text or standard `application/x-www-form-urlencoded` string containing parameter `q`.
  * Response Envelope:
    ```json
    {
      "responseData": {
        "language": "en",
        "confidence": 1.0
      },
      "responseDetails": null,
      "responseStatus": 200
    }
    ```
* **`POST /rank`** or **`GET /rank?q=<text>`**: Retrieve full confidence list.
  * Response Envelope:
    ```json
    {
      "responseData": [
        ["en", 1.0],
        ["fr", 0.0],
        ["es", 0.0]
      ],
      "responseDetails": null,
      "responseStatus": 200
    }
    ```
* **`GET /demo`**: Returns the interactive web UI sandbox.

---

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
