# langid-go

[![Go Reference](https://pkg.go.dev/badge/github.com/ilpy20/langid-go.svg)](https://pkg.go.dev/github.com/ilpy20/langid-go)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ilpy20/langid-go)](https://go.dev/)
[![License](https://img.shields.io/github/license/ilpy20/langid-go)](./LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/ilpy20/langid-go/test.yml?branch=master)](https://github.com/ilpy20/langid-go/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/ilpy20/langid-go)](https://goreportcard.com/report/github.com/ilpy20/langid-go)

`langid-go` is a high-performance Go natural language identifier pre-trained on 97 languages. By leveraging Go's concurrency primitives (`sync.Pool`) and a flat-array "sparse set" architecture, this port achieves **zero-allocation inference** in the hot loop, making it extremely fast and suitable for high-throughput stream processing.

---

## Key Features

- **Pre-trained on 97 languages** (using standard ISO 639-1 two-letter codes).
- **Embedded Model**: Completely CGO-free, requiring no external file or runtime dependencies. The default model is compiled directly into the binary via `go:embed`.
- **Zero-Allocation Hot-Loop**: Designed for real-time chatbot gateways and NLP pipelines with garbage collection (GC) overhead eliminated on hot execution paths.
- **Language Subsetting**: Restrict language predictions to a known subset of languages for even greater accuracy and classification speeds.
- **Probability Normalization**: Standardizes raw log-scores into a standard 0.0 - 1.0 probability distribution.
- **Feature-Rich CLI**: Supports interactive REPL, document, line-by-line, batch, URL, and local HTTP service modes.

---

## Quick Start

### Installation
```bash
go get github.com/ilpy20/langid-go
```

### Basic Library Usage
```go
package main

import (
	"fmt"
	"github.com/ilpy20/langid-go"
)

func main() {
	// Identify text using the default embedded model
	res, err := langid.Classify("This is a short English sentence")
	if err == nil {
		// res.Language == "en", res.Score is the raw negative log probability
		fmt.Printf("Predicted Language: %s (Log Score: %.2f)\n", res.Language, res.Score)
	}
}
```

---

## Documentation Index

Explore our comprehensive guides for specialized usage patterns and architectural deep dives:

| Document | Description |
| --- | --- |
| 📖 [Library API Guide](docs/library_usage.md) | Detailed programmatical guide to subsetting, score normalization, file helpers, `urlclass`, and `service` APIs. |
| 💻 [CLI Usage Guide](docs/cli_usage.md) | How to build and run the CLI for REPL, batch file streaming, URL parsing, and running the HTTP microservice. |
| 🧠 [Model Training & Conversion](docs/model_training.md) | Technical decisions, step-by-step workflow for training custom models in Python and compiling them to `.lidg` binary files. |
| ⚡ [Architecture & Motivation](docs/architecture_and_background.md) | Why `langid-go` exists, comparison with alternative libraries (`lingua-go`, `whatlanggo`), and deep-dive into zero-allocation pooling. |

---

## Compatibility Matrix

`langid-go` is designed as a fully featured, production-ready superset of the original ecosystem libraries:

| Feature | langid.py | langid.c | langid.js | langid-go |
| --- | --- | --- | --- | --- |
| **Default 97-language model** | yes | yes | yes | **yes** |
| **Custom model loading** | yes | yes | via generated JS | **yes (via `.lidg` binary)** |
| **Classify text** | yes | yes | yes | **yes** |
| **Return raw log-score** | yes | no | yes (as ranks) | **yes** |
| **Rank all languages** | yes | no | yes | **yes** |
| **Normalize probabilities** | yes | no | no | **yes** |
| **Language subsetting** | yes | no | no | **yes** |
| **File helper API** | yes | CLI only | no | **yes** |
| **CLI document mode** | yes | yes | no | **yes** |
| **CLI line mode** | yes | yes | no | **yes** |
| **CLI batch mode** | yes | yes | no | **yes** |
| **Python-compatible flags** | yes | partial | no | **yes** |
| **URL classification** | yes | no | no | **yes** |
| **HTTP service mode** | yes | no | no | **yes** |
| **Web browser demo** | yes | no | yes | **yes** |
| **Training tools** | yes | no | no | *Planned Future Feature (TODO)* |

---

## Contributing

We welcome community feedback, issue reports, and pull requests! Please feel free to open a ticket on our GitHub issues page. See our planned roadmap features in [TODO.md](./TODO.md).

---

## Acknowledgements and References

`langid.go` is a port of the Naive Bayes / DFA language identification algorithm originally created by Marco Lui and Timothy Baldwin. 

* [1] Lui, Marco and Timothy Baldwin (2011) *Cross-domain Feature Selection for Language Identification*, In Proceedings of the Fifth International Joint Conference on Natural Language Processing (IJCNLP 2011), Chiang Mai, Thailand, pp. 553—561. [PDF](http://www.aclweb.org/anthology/I11-1062)
* [2] Lui, Marco and Timothy Baldwin (2012) *langid.py: An Off-the-shelf Language Identification Tool*, In Proceedings of the 50th Annual Meeting of the Association for Computational Linguistics (ACL 2012), Demo Session, Jeju, Republic of Korea. [PDF](http://www.aclweb.org/anthology/P12-3005)

---

## License

This project is licensed under the BSD-2 Clause License - see the [LICENSE](./LICENSE) file for details.
