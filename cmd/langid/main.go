package main

import (
	"bufio"
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

func main() {
	modelPath := flag.String("model", "", "path to .lidg model (optional, uses default if omitted)")
	lineMode := flag.Bool("l", false, "line mode: classify each input line")
	batchMode := flag.Bool("b", false, "batch mode: treat stdin lines as file paths to classify")
	langs := flag.String("langs", "", "comma-separated set of target ISO639 language codes (e.g en,de)")
	dist := flag.Bool("d", false, "show full distribution over languages (rank mode)")
	normalize := flag.Bool("n", false, "normalize confidence scores to probability values (0.0 to 1.0)")
	flag.Parse()

	if *lineMode && *batchMode {
		fmt.Fprintln(os.Stderr, "cannot specify both -l and -b at the same time")
		os.Exit(1)
	}

	var id *langid.Identifier
	var err error

	if *modelPath != "" {
		id, err = langid.LoadModel(*modelPath)
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

	if *langs != "" {
		langList := strings.Split(*langs, ",")
		for i := range langList {
			langList[i] = strings.TrimSpace(langList[i])
		}
		if err := id.KeepOnly(langList...); err != nil {
			fmt.Fprintf(os.Stderr, "filter languages: %v\n", err)
			os.Exit(1)
		}
	}

	if *batchMode {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			path := strings.TrimSpace(s.Text())
			if path == "" {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("%s,NOSUCHFILE\n", path)
				continue
			}
			fmt.Printf("%s,", path)
			processInput(id, data, *dist, *normalize)
		}
		if err := s.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *lineMode {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			line := s.Bytes()
			processInput(id, line, *dist, *normalize)
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
	processInput(id, data, *dist, *normalize)
}
