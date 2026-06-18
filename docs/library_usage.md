# Library API Usage Guide

This document is a comprehensive guide to using the `langid-go` programmatic API in your Go projects. It covers basic classification, stateful identifier instantiation, language subsetting, file helpers, URL classification, and setting up the HTTP service.

---

## 1. Quick Start

Add `langid-go` to your project:
```bash
go get github.com/ilpy20/langid-go
```

The simplest way to identify text is using the package-level `Classify` function, which lazily loads the default embedded model on its first invocation.

```go
package main

import (
	"fmt"
	"log"

	"github.com/ilpy20/langid-go"
)

func main() {
	res, err := langid.Classify("This is a short English sentence")
	if err != nil {
		log.Fatalf("failed to classify: %v", err)
	}

	// Output: Language: en, Raw Log Score: -102.5
	fmt.Printf("Language: %s, Raw Log Score: %.2f\n", res.Language, res.Score)
}
```

---

## 2. Stateful Identifier (`Identifier`)

For highly concurrent applications, or when you need to configure language subsets and score normalization, you should instantiate a stateful `Identifier`.

An `Identifier` is completely concurrency-safe. Multiple goroutines can invoke classification methods concurrently.

### Creating an Identifier
You can create an `Identifier` using the default embedded model or by loading a custom `.lidg` model file.

```go
// Create using default embedded model
id, err := langid.NewDefaultIdentifier()

// Create by loading a custom .lidg model file
id, err := langid.LoadModel("path/to/my_model.lidg")
```

---

## 3. Advanced Features

### Language Subsetting
If you are working with a known domain or standard set of languages, you can restrict the classifier's search space. This increases classification speed and drastically improves prediction accuracy.

> [!NOTE]
> Subset changes are thread-safe and atomic. In-flight classifications continue using the model snapshot they started with, while subsequent calls transition to the new subset.

```go
package main

import (
	"fmt"
	"github.com/ilpy20/langid-go"
)

func main() {
	id, _ := langid.NewDefaultIdentifier()

	// Restrict classifier search to only English, French, Spanish, German, and Italian
	err := id.SetLanguages("en", "fr", "es", "de", "it")
	if err != nil {
		fmt.Printf("Unsupported language in subset request: %v\n", err)
		return
	}

	res, _ := id.IdentifyString("Bonjour tout le monde")
	fmt.Printf("Language: %s\n", res.Language) // Output: fr

	// Reset to all 97 supported default languages
	id.ResetLanguages()
}
```

### Probability Normalization
By default, Naive Bayes scores are returned as negative log-probabilities (lower absolute numbers mean higher confidence). You can normalize a sorted slice of rankings to obtain standard probability percentages (0.0 to 1.0).

```go
package main

import (
	"fmt"
	"github.com/ilpy20/langid-go"
)

func main() {
	id, _ := langid.NewDefaultIdentifier()
	_ = id.SetLanguages("en", "fr", "es")

	// Retrieve sorted list of rankings for all active languages
	rankings, _ := id.RankString("Bonjour tout le monde")

	// Normalize scores in-place to represent a 0.0 - 1.0 probability distribution
	langid.Normalize(rankings)

	for _, r := range rankings {
		fmt.Printf("%s: %.2f%%\n", r.Language, r.Score*100)
	}
	// Output:
	// fr: 99.98%
	// en: 0.02%
	// es: 0.00%
}
```

---

## 4. Helper APIs

### File Classification Helpers
`langid-go` provides native file helpers at both the package level (using the default model) and instance level to classify local documents efficiently without sacrificing error context.

```go
// Package-level helpers
res, err := langid.IdentifyFile("document.txt")
rankings, err := langid.RankFile("document.txt")

// Instance-level helpers
res, err := id.IdentifyFile("document.txt")
rankings, err := id.RankFile("document.txt")
```

---

## 5. Webpage URL Classification (`urlclass`)

The `urlclass` package lets you programmatic fetch, parse, and classify the primary text content of web pages. It features built-in timeout management.

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

	// Fetch page, strip markup, and identify language with a 5-second timeout limit
	res, bytesFetched, err := client.ClassifyURL("https://example.com", 5*time.Second)
	if err != nil {
		fmt.Printf("URL classification failed: %v\n", err)
		return
	}

	fmt.Printf("Fetched %d bytes. Predicted Language: %s (Score: %.2f)\n", bytesFetched, res.Language, res.Score)
}
```

---

## 6. HTTP Microservice API (`service`)

The `service` package provides an HTTP router wrapping `langid` to expose classification endpoints. It handles requests concurrently and comes with a built-in interactive sandbox.

```go
package main

import (
	"log"

	"github.com/ilpy20/langid-go"
	"github.com/ilpy20/langid-go/service"
)

func main() {
	id, _ := langid.NewDefaultIdentifier()
	srv := service.NewServer(id)

	log.Println("Starting HTTP API service on http://127.0.0.1:9008...")
	if err := srv.Start("127.0.0.1", 9008); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
```
