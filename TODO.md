# langid-go TODO

This document details future roadmap items, enhancements, and planned features for `langid-go`.

---

## 1. Go-Native Model Training (Planned Future Feature)
Currently, model training is delegated to Python (`langid.py/langid/train`), requiring model conversion via `scripts/convert_model.py`. Introducing a Go-native model training pipeline remains a key future milestone once suitable libraries are established.

- [ ] **Corpus Indexer**: Native directory crawler traversing corpus folder trees (e.g., `/corpus/{lang}/{doc}`) and building language class collections.
- [ ] **Byte-Level N-Gram Tokenizer**: A highly optimized sliding window tokenizer (sizes 1–4) designed to process large text files and count document frequencies with bounded memory overhead.
- [ ] **Statistical Solver & Feature Selection**:
  - [ ] Implementation of Shannon Entropy and Information Gain (IG) weight calculations:
    $$IG(F, C) = H(C) - H(C|F)$$
  - [ ] Implementation of language-difference (LD) feature selection to select the top $N$ discriminating features for each language.
- [ ] **Aho-Corasick Compiler**: Compiler that constructs a keyword trie from selected n-grams, resolves failure/output links, and serializes them into a single-dimensional `tk_nextmove` transition array compatible with `.lidg`.
- [ ] **Naive Bayes Parameter Estimator**: Native training routine to calculate Naive Bayes log-likelihood parameters (`nb_pc` and `nb_ptc`).

---

## 2. Research & Library Benchmarking
To support native training without rebuilding common math packages from scratch, investigate and benchmark the following Go ecosystems:
- [ ] **`gonum/gonum`**: Benchmark `gonum` for multi-dimensional matrix operations and Shannon entropy calculations to evaluate memory efficiency and performance compared to Python's C-accelerated `numpy` and `scipy`.
- [ ] **Keyword Scanning**: Evaluate pure Go implementations of Aho-Corasick compilers to see if any can be adapted to compile the specialized transition tables required by `langid-go`'s state machine.

---

## 3. Library & CLI Enhancements
- [ ] **Micro-Optimizations**: Analyze hot-loop memory caches and investigate if CPU cache alignment or alternative sparse-set layouts can shave off additional execution nanoseconds.
- [ ] **Go 1.23+ Iterators**: Implement custom iterators for line-by-line file classifications in standard Go workflows.
- [ ] **Extended Model Support**: Add a CLI subcommand or helper tool for merging or splitting custom `.lidg` files.

---

## 4. Operational & CI/CD Tooling
- [ ] **Continuous Benchmarking**: Configure a GitHub Actions workflow that executes `go test -bench` on every commit and flags performance regressions.
- [ ] **Dockerized Service**: Package `langid-go`'s HTTP service mode into a minimal scratch-based Docker image for easy container orchestration.
