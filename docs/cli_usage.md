# CLI Usage Guide

`langid-go` includes a feature-rich, high-performance command-line tool. It is fully backwards-compatible with the CLI behaviors of `langid.py` and `langid.c`, while offering modern extensions like multiple output formats, URL classification, and a local interactive sandbox.

---

## 1. Build and Setup

To compile the `langid` command-line executable:
```bash
go build -o langid ./cmd/langid
```

This generates a native static binary `langid` with zero runtime dependencies.

---

## 2. Command Options Reference

```text
Usage of langid:
  -m, --model string
    	path to .lidg model (optional, uses default if omitted)
  -l, --langs string
    	comma-separated set of target ISO639 language codes (e.g en,de)
      --line
    	line mode: classify each input line (legacy alias: -l)
  -b, --batch
    	batch mode: treat stdin lines or CLI arguments as file paths to classify
  -d, --dist
    	show full distribution over languages (rank mode)
  -n, --normalize
    	normalize confidence scores to probability values (0.0 to 1.0)
  -f, --format string
    	output format for batch mode: classic, csv, or jsonl (default "classic")
      --ignore-missing
    	silently skip missing or unreadable files in batch mode
      --max-bytes int
    	maximum bytes read per stdin, file, request, or URL body (default 4194304)
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

---

## 3. Modes of Operation

### 1. Standard Pipe Mode
Reads a single document from standard input and prints the best prediction.
```bash
$ ./langid -n <<< "This is a simple sentence."
('en', 1.0000)
```

### 2. Interactive REPL Mode
Running `./langid` with standard input attached to a terminal automatically boots a Python-style interactive shell.
```bash
$ ./langid -n
>>> Hello World
('en', 1.0000)
>>> Bonjour tout le monde
('fr', 1.0000)
```

### 3. Line-by-Line Mode (`--line`)
Processes standard input line-by-line, treating each line as a separate document.
```bash
$ printf "hello world\nbonjour tout le monde\n" | ./langid --line
('en', -102.5)
('fr', -105.1)
```

### 4. Batch Mode (`--batch` / `-b`)
Reads files and classifies them. You can pass file paths directly as command-line arguments, or pipe a list of file paths to standard input.

#### Passing Files Directly
```bash
./langid --batch --format csv file1.txt file2.txt
```

#### Piping File Lists
```bash
find . -name "*.md" | ./langid -b -n --format jsonl
```

#### Batch Output Formats
Control the batch outputs with the `--format` flag:
- **`classic` (default)**: Matches the original C implementation (`langid.c`) output format: `path,('lang', score)`
- **`csv`**: Matches the standard Python implementation (`langid.py`) batch output format:
  - Standard: Outputs `path,lang,score` (no header row).
  - Distribution (`-d`): Outputs a header followed by scores for all supported columns:
    ```text
    path,en,fr,es,de,...
    file1.txt,-105.1,-240.2,...
    ```
- **`jsonl`**:
  - Standard:
    ```json
    {"path":"file1.txt","language":"en","confidence":-105.1}
    ```
  - Distribution (`-d`):
    ```json
    {"path":"file1.txt","ranking":[{"language":"en","score":-105.1},{"language":"fr","score":-240.2},...]}
    ```

#### Missing Files
By default, batch mode reports `"NOSUCHFILE"` for unreadable or missing files. Add `--ignore-missing` to silently skip them.
```bash
./langid --batch --ignore-missing file1.txt missing_file.txt
```

### 5. URL Classification (`--url` / `-u`)
Fetches a webpage, extracts its plain text, and identifies the language directly from the terminal.
```bash
$ ./langid --url "https://example.com" -n
https://example.com 1256 ('en', 1.0000)
```
*(Prints: `URL`, `Length in Bytes`, and `Result`)*

---

## 4. HTTP Service Mode (`--serve` / `--demo`)

Run `langid` as a microservice on your local network:
```bash
# Start the HTTP server on port 9008
./langid --serve --port 9008

# Start the server and automatically launch the interactive UI sandbox in your default browser
./langid --demo
```

### API Endpoints Specifications

#### `POST /detect` | `GET /detect?q=<query>`
Identifies the best language for the given text.

- **Request Body**: Raw text in POST body, or `application/x-www-form-urlencoded` string with parameter `q`.
- **Response Structure (JSON)**:
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

#### `POST /rank` | `GET /rank?q=<query>`
Ranks all supported languages.

- **Request Body**: Raw text in POST body, or `application/x-www-form-urlencoded` string with parameter `q`.
- **Response Structure (JSON)**:
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

#### `GET /demo`
Serves the responsive, interactive HTML dashboard sandbox to test classification dynamically in real-time.
