package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ilpy20/langid-go"
)

func processInput(id *langid.Identifier, data []byte, dist, normalize bool) {
	if dist {
		results, err := id.RankBytes(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rank: %v\n", err)
			return
		}
		if normalize {
			langid.Normalize(results)
		}
		fmt.Print("[")
		for i, r := range results {
			if i > 0 {
				fmt.Print(", ")
			}
			if normalize {
				fmt.Printf("('%s', %.4f)", r.Language, r.Score)
			} else {
				fmt.Printf("('%s', %.1f)", r.Language, r.Score)
			}
		}
		fmt.Println("]")
	} else {
		if normalize {
			results, err := id.RankBytes(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rank: %v\n", err)
				return
			}
			langid.Normalize(results)
			fmt.Printf("('%s', %.4f)\n", results[0].Language, results[0].Score)
		} else {
			res, err := id.IdentifyBytes(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "classify: %v\n", err)
				return
			}
			fmt.Printf("('%s', %.1f)\n", res.Language, res.Score)
		}
	}
}

func preprocessArgs(args []string) []string {
	var preprocessed []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-l" || arg == "--l" {
			// Check if there is a next argument and it doesn't start with "-"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// User provided a value: this is language-filtering (Python style)
				preprocessed = append(preprocessed, "--langs", args[i+1])
				i++
			} else {
				// No value following, or next arg is a flag: this is line mode (legacy Go style)
				preprocessed = append(preprocessed, "--line")
			}
		} else if strings.HasPrefix(arg, "-l=") || strings.HasPrefix(arg, "--l=") {
			// User used -l=val format. This is language filtering.
			val := strings.TrimPrefix(strings.TrimPrefix(arg, "-l="), "--l=")
			preprocessed = append(preprocessed, "--langs", val)
		} else {
			preprocessed = append(preprocessed, arg)
		}
	}
	return preprocessed
}

func main() {
	var (
		modelPath string
		mPath     string
		lineMode  bool
		batchMode bool
		bMode     bool
		langs     string
		dist      bool
		dMode     bool
		normalize bool
		nMode     bool
		format    string
		fFormat   string
	)

	flag.StringVar(&modelPath, "model", "", "path to .lidg model (optional, uses default if omitted)")
	flag.StringVar(&mPath, "m", "", "path to .lidg model (alias for -model)")
	flag.BoolVar(&lineMode, "line", false, "line mode: classify each input line (legacy alias: -l)")
	flag.BoolVar(&batchMode, "batch", false, "batch mode: treat stdin lines as file paths to classify")
	flag.BoolVar(&bMode, "b", false, "batch mode: treat stdin lines as file paths to classify (alias for -batch)")
	flag.StringVar(&langs, "langs", "", "comma-separated set of target ISO639 language codes (e.g en,de)")
	flag.BoolVar(&dist, "dist", false, "show full distribution over languages (rank mode)")
	flag.BoolVar(&dMode, "d", false, "show full distribution over languages (rank mode) (alias for -dist)")
	flag.BoolVar(&normalize, "normalize", false, "normalize confidence scores to probability values (0.0 to 1.0)")
	flag.BoolVar(&nMode, "n", false, "normalize confidence scores to probability values (0.0 to 1.0) (alias for -normalize)")
	flag.StringVar(&format, "format", "classic", "output format for batch mode: classic, csv, or jsonl")
	flag.StringVar(&fFormat, "f", "", "output format for batch mode: classic, csv, or jsonl (alias for -format)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "  -m, -model string\n    \tpath to .lidg model (optional, uses default if omitted)")
		fmt.Fprintln(os.Stderr, "  -l, --langs string\n    \tcomma-separated set of target ISO639 language codes (e.g en,de)")
		fmt.Fprintln(os.Stderr, "      --line\n    \tline mode: classify each input line (legacy alias: -l)")
		fmt.Fprintln(os.Stderr, "  -b, --batch\n    \tbatch mode: treat stdin lines as file paths to classify")
		fmt.Fprintln(os.Stderr, "  -d, --dist\n    \tshow full distribution over languages (rank mode)")
		fmt.Fprintln(os.Stderr, "  -n, --normalize\n    \tnormalize confidence scores to probability values (0.0 to 1.0)")
		fmt.Fprintln(os.Stderr, "  -f, --format string\n    \toutput format for batch mode: classic, csv, or jsonl (default \"classic\")")
	}

	os.Args = append(os.Args[:1], preprocessArgs(os.Args[1:])...)
	flag.Parse()

	actualModelPath := modelPath
	if mPath != "" {
		actualModelPath = mPath
	}

	formatSpecified := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "format" || f.Name == "f" {
			formatSpecified = true
		}
	})

	actualFormat := "classic"
	if fFormat != "" {
		actualFormat = fFormat
	} else if format != "" {
		actualFormat = format
	}

	if actualFormat != "classic" && actualFormat != "csv" && actualFormat != "jsonl" {
		fmt.Fprintf(os.Stderr, "unknown output format: %s\n", actualFormat)
		os.Exit(1)
	}

	actualBatchMode := batchMode || bMode || formatSpecified
	actualDist := dist || dMode
	actualNormalize := normalize || nMode

	if lineMode && actualBatchMode {
		fmt.Fprintln(os.Stderr, "cannot specify both line mode and batch mode at the same time")
		os.Exit(1)
	}

	var id *langid.Identifier
	var err error

	if actualModelPath != "" {
		id, err = langid.LoadModel(actualModelPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load model: %v\n", err)
			os.Exit(1)
		}
	} else {
		id, err = langid.NewDefaultIdentifier()
		if err != nil {
			fmt.Fprintf(os.Stderr, "load default model: %v\n", err)
			os.Exit(1)
		}
	}

	if langs != "" {
		langList := strings.Split(langs, ",")
		for i := range langList {
			langList[i] = strings.TrimSpace(langList[i])
		}
		if err := id.KeepOnly(langList...); err != nil {
			fmt.Fprintf(os.Stderr, "filter languages: %v\n", err)
			os.Exit(1)
		}
	}

	if actualBatchMode {
		var csvWriter *csv.Writer
		var classes []string
		if actualFormat == "csv" {
			csvWriter = csv.NewWriter(os.Stdout)
			defer csvWriter.Flush()
			if actualDist {
				classes = id.Classes()
				header := append([]string{"path"}, classes...)
				_ = csvWriter.Write(header)
			}
		}

		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			path := strings.TrimSpace(s.Text())
			if path == "" {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				if actualFormat == "csv" {
					if actualDist {
						row := make([]string, len(classes)+1)
						row[0] = path
						row[1] = "NOSUCHFILE"
						_ = csvWriter.Write(row)
					} else {
						_ = csvWriter.Write([]string{path, "NOSUCHFILE", ""})
					}
				} else if actualFormat == "jsonl" {
					fmt.Printf("{\"path\":%q,\"error\":\"NOSUCHFILE\"}\n", path)
				} else {
					fmt.Printf("%s,NOSUCHFILE\n", path)
				}
				continue
			}

			switch actualFormat {
			case "csv":
				if actualDist {
					results, err := id.RankBytes(data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "rank: %v\n", err)
						continue
					}
					if actualNormalize {
						langid.Normalize(results)
					}
					scoreMap := make(map[string]string)
					for _, r := range results {
						if actualNormalize {
							scoreMap[r.Language] = fmt.Sprintf("%.4f", r.Score)
						} else {
							scoreMap[r.Language] = fmt.Sprintf("%.1f", r.Score)
						}
					}
					row := make([]string, 0, len(classes)+1)
					row = append(row, path)
					for _, c := range classes {
						row = append(row, scoreMap[c])
					}
					_ = csvWriter.Write(row)
				} else {
					var lang string
					var confidence float64
					if actualNormalize {
						results, err := id.RankBytes(data)
						if err != nil {
							fmt.Fprintf(os.Stderr, "rank: %v\n", err)
							continue
						}
						langid.Normalize(results)
						lang = results[0].Language
						confidence = results[0].Score
					} else {
						res, err := id.IdentifyBytes(data)
						if err != nil {
							fmt.Fprintf(os.Stderr, "classify: %v\n", err)
							continue
						}
						lang = res.Language
						confidence = res.Score
					}

					var confStr string
					if actualNormalize {
						confStr = fmt.Sprintf("%.4f", confidence)
					} else {
						confStr = fmt.Sprintf("%.1f", confidence)
					}
					_ = csvWriter.Write([]string{path, lang, confStr})
				}

			case "jsonl":
				if actualDist {
					results, err := id.RankBytes(data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "rank: %v\n", err)
						continue
					}
					if actualNormalize {
						langid.Normalize(results)
					}
					type jsonlRankItem struct {
						Language string  `json:"language"`
						Score    float64 `json:"score"`
					}
					ranking := make([]jsonlRankItem, len(results))
					for i, r := range results {
						ranking[i] = jsonlRankItem{
							Language: r.Language,
							Score:    r.Score,
						}
					}
					type jsonlDistRow struct {
						Path    string          `json:"path"`
						Ranking []jsonlRankItem `json:"ranking"`
					}
					row := jsonlDistRow{
						Path:    path,
						Ranking: ranking,
					}
					b, err := json.Marshal(row)
					if err != nil {
						fmt.Fprintf(os.Stderr, "json marshal: %v\n", err)
						continue
					}
					fmt.Println(string(b))
				} else {
					var lang string
					var confidence float64
					if actualNormalize {
						results, err := id.RankBytes(data)
						if err != nil {
							fmt.Fprintf(os.Stderr, "rank: %v\n", err)
							continue
						}
						langid.Normalize(results)
						lang = results[0].Language
						confidence = results[0].Score
					} else {
						res, err := id.IdentifyBytes(data)
						if err != nil {
							fmt.Fprintf(os.Stderr, "classify: %v\n", err)
							continue
						}
						lang = res.Language
						confidence = res.Score
					}

					type jsonlClassifyRow struct {
						Path       string  `json:"path"`
						Language   string  `json:"language"`
						Confidence float64 `json:"confidence"`
					}
					row := jsonlClassifyRow{
						Path:       path,
						Language:   lang,
						Confidence: confidence,
					}
					b, err := json.Marshal(row)
					if err != nil {
						fmt.Fprintf(os.Stderr, "json marshal: %v\n", err)
						continue
					}
					fmt.Println(string(b))
				}

			default: // "classic"
				fmt.Printf("%s,", path)
				processInput(id, data, actualDist, actualNormalize)
			}
		}
		if err := s.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if lineMode {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			line := s.Bytes()
			processInput(id, line, actualDist, actualNormalize)
		}
		if err := s.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}
	processInput(id, data, actualDist, actualNormalize)
}
