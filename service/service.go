package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ilpy20/langid-go"
)

const DefaultMaxRequestBytes int64 = 4 << 20

// ResponseEnvelope represents the shared standard response format for API results.
type ResponseEnvelope struct {
	ResponseData    any     `json:"responseData"`
	ResponseDetails *string `json:"responseDetails"`
	ResponseStatus  int     `json:"responseStatus"`
}

// Server wraps the langid web service.
type Server struct {
	id              *langid.Identifier
	server          *http.Server
	maxRequestBytes int64
}

// NewServer creates a new instance of the Server with the provided identifier.
func NewServer(id *langid.Identifier) *Server {
	return &Server{
		id:              id,
		maxRequestBytes: DefaultMaxRequestBytes,
	}
}

// SetMaxRequestBytes sets the maximum size accepted for POST and PUT request bodies.
// Non-positive values disable the limit.
func (s *Server) SetMaxRequestBytes(n int64) {
	if s == nil {
		return
	}
	s.maxRequestBytes = n
}

// NewHandler creates and configures the HTTP router for /detect, /rank, and /demo endpoints.
func (s *Server) NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/detect", s.handleDetect)
	mux.HandleFunc("/rank", s.handleRank)
	mux.HandleFunc("/demo", s.handleDemo)
	mux.HandleFunc("/", s.handleNotFound)
	return mux
}

// Start runs the HTTP service listening on the specified host and port.
func (s *Server) Start(host string, port int) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	s.server = &http.Server{
		Addr:              addr,
		Handler:           s.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	fmt.Printf("Starting langid service on http://%s\n", addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	data, ok := s.extractQuery(w, r)
	if !ok {
		return
	}

	results, err := s.id.RankString(data)
	if err != nil {
		writeJSONEnvelope(w, http.StatusInternalServerError, nil, pointerToString(err.Error()))
		return
	}

	langid.Normalize(results)

	var responseData map[string]any
	if len(results) > 0 {
		responseData = map[string]any{
			"language":   results[0].Language,
			"confidence": results[0].Score,
		}
	} else {
		responseData = map[string]any{
			"language":   "",
			"confidence": 0.0,
		}
	}

	writeJSONEnvelope(w, http.StatusOK, responseData, nil)
}

func (s *Server) handleRank(w http.ResponseWriter, r *http.Request) {
	data, ok := s.extractQuery(w, r)
	if !ok {
		return
	}

	results, err := s.id.RankString(data)
	if err != nil {
		writeJSONEnvelope(w, http.StatusInternalServerError, nil, pointerToString(err.Error()))
		return
	}

	langid.Normalize(results)

	responseData := make([][2]any, len(results))
	for i, r := range results {
		responseData[i] = [2]any{r.Language, r.Score}
	}

	writeJSONEnvelope(w, http.StatusOK, responseData, nil)
}

func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(queryForm))
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeJSONEnvelope(w, http.StatusNotFound, nil, pointerToString("Not found"))
}

func (s *Server) extractQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	var data string
	switch r.Method {
	case http.MethodGet:
		if !r.URL.Query().Has("q") {
			writeJSONEnvelope(w, http.StatusOK, nil, nil)
			return "", false
		}
		data = r.URL.Query().Get("q")

	case http.MethodPost:
		bodyBytes, err := s.readBody(w, r)
		if err != nil {
			s.writeBodyReadError(w, err)
			return "", false
		}
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
			values, err := url.ParseQuery(string(bodyBytes))
			if err == nil && values.Has("q") {
				data = values.Get("q")
			} else {
				data = string(bodyBytes)
			}
		} else {
			data = string(bodyBytes)
		}

	case http.MethodPut:
		bodyBytes, err := s.readBody(w, r)
		if err != nil {
			s.writeBodyReadError(w, err)
			return "", false
		}
		data = string(bodyBytes)

	default:
		writeJSONEnvelope(w, http.StatusMethodNotAllowed, nil, pointerToString(fmt.Sprintf("%s not allowed", r.Method)))
		return "", false
	}
	return data, true
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	reader := r.Body
	if s.maxRequestBytes > 0 {
		reader = http.MaxBytesReader(w, r.Body, s.maxRequestBytes)
	}
	return io.ReadAll(reader)
}

func (s *Server) writeBodyReadError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeJSONEnvelope(w, http.StatusRequestEntityTooLarge, nil, pointerToString(fmt.Sprintf("request body exceeds %d bytes", maxErr.Limit)))
		return
	}
	writeJSONEnvelope(w, http.StatusBadRequest, nil, pointerToString("failed to read body"))
}

func writeJSONEnvelope(w http.ResponseWriter, status int, data any, details *string) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.WriteHeader(status)

	envelope := ResponseEnvelope{
		ResponseData:    data,
		ResponseDetails: details,
		ResponseStatus:  status,
	}

	encoder := json.NewEncoder(w)
	_ = encoder.Encode(envelope)
}

func pointerToString(s string) *string {
	return &s
}

const queryForm = `<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Language Identifier</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
      :root {
        --bg-color: #0b0f19;
        --card-bg: rgba(22, 28, 45, 0.6);
        --border-color: rgba(255, 255, 255, 0.08);
        --text-primary: #f3f4f6;
        --text-secondary: #9ca3af;
        --accent-color: #3b82f6;
        --accent-glow: rgba(59, 130, 246, 0.15);
        --success-color: #10b981;
      }

      * {
        box-sizing: border-box;
        margin: 0;
        padding: 0;
      }

      body {
        background-color: var(--bg-color);
        color: var(--text-primary);
        font-family: 'Outfit', sans-serif;
        min-height: 100vh;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 2rem;
        background-image: 
          radial-gradient(at 0% 0%, rgba(59, 130, 246, 0.1) 0px, transparent 50%),
          radial-gradient(at 100% 100%, rgba(16, 185, 129, 0.05) 0px, transparent 50%);
      }

      .container {
        width: 100%;
        max-width: 600px;
        background: var(--card-bg);
        backdrop-filter: blur(12px);
        -webkit-backdrop-filter: blur(12px);
        border: 1px solid var(--border-color);
        border-radius: 24px;
        padding: 2.5rem;
        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
      }

      h1 {
        font-size: 2rem;
        font-weight: 700;
        margin-bottom: 0.5rem;
        background: linear-gradient(135deg, #fff 0%, #93c5fd 100%);
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
        text-align: center;
      }

      .subtitle {
        color: var(--text-secondary);
        font-size: 0.95rem;
        text-align: center;
        margin-bottom: 2rem;
      }

      .textarea-wrapper {
        position: relative;
        margin-bottom: 2rem;
      }

      textarea {
        width: 100%;
        height: 140px;
        background: rgba(15, 23, 42, 0.6);
        border: 1px solid var(--border-color);
        border-radius: 16px;
        padding: 1.2rem;
        color: var(--text-primary);
        font-family: inherit;
        font-size: 1.05rem;
        resize: none;
        outline: none;
        transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
      }

      textarea:focus {
        border-color: var(--accent-color);
        box-shadow: 0 0 0 4px var(--accent-glow);
      }

      .results-container {
        border-top: 1px solid var(--border-color);
        padding-top: 1.5rem;
      }

      .results-title {
        font-size: 0.85rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--text-secondary);
        margin-bottom: 1rem;
        font-weight: 600;
      }

      .rank-list {
        display: flex;
        flex-direction: column;
        gap: 0.8rem;
      }

      .rank-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1.2rem;
        background: rgba(255, 255, 255, 0.02);
        border: 1px solid var(--border-color);
        border-radius: 12px;
        transition: all 0.2s ease;
      }

      .rank-item:first-child {
        background: rgba(59, 130, 246, 0.08);
        border-color: rgba(59, 130, 246, 0.3);
      }

      .rank-item:hover {
        background: rgba(255, 255, 255, 0.04);
        transform: translateX(4px);
      }

      .lang-badge {
        font-weight: 600;
        font-size: 1.1rem;
        color: var(--text-primary);
        display: flex;
        align-items: center;
        gap: 0.5rem;
      }

      .rank-item:first-child .lang-badge {
        color: #60a5fa;
      }

      .confidence {
        font-family: monospace;
        font-size: 1rem;
        color: var(--success-color);
        font-weight: 600;
      }

      #manualSubmit {
        display: inline-block;
        width: 100%;
        padding: 1rem;
        background: var(--accent-color);
        color: white;
        border: none;
        border-radius: 12px;
        font-weight: 600;
        cursor: pointer;
        transition: background 0.2s;
        margin-top: 1rem;
      }

      #manualSubmit:hover {
        background: #2563eb;
      }
    </style>
    <script type="text/javascript" charset="utf-8">
      document.addEventListener("DOMContentLoaded", () => {
        const typer = document.getElementById("typerArea");
        const table = document.getElementById("rankTable");
        const submit = document.getElementById("manualSubmit");
        const listContainer = document.getElementById("rankList");

        if (submit) submit.remove();
        if (table) table.style.display = "none";

        let debounceTimeout;

        typer.addEventListener("input", () => {
          clearTimeout(debounceTimeout);
          debounceTimeout = setTimeout(async () => {
            const contents = typer.value.trim();
            if (contents.length === 0) {
              if (table) table.style.display = "none";
              return;
            }

            try {
              const response = await fetch("/rank", {
                method: "POST",
                headers: {
                  "Content-Type": "application/x-www-form-urlencoded"
                },
                body: "q=" + encodeURIComponent(contents)
              });
              const data = await response.json();

              if (data && data.responseData && data.responseData.length > 0) {
                listContainer.innerHTML = "";
                
                const limit = Math.min(data.responseData.length, 5);
                for (let i = 0; i < limit; i++) {
                  const lang = data.responseData[i][0];
                  const conf = Number(data.responseData[i][1]).toFixed(4);

                  const item = document.createElement("div");
                  item.className = "rank-item";
                  
                  const langBadge = document.createElement("span");
                  langBadge.className = "lang-badge";
                  langBadge.textContent = lang;

                  const confSpan = document.createElement("span");
                  confSpan.className = "confidence";
                  confSpan.textContent = conf;

                  item.appendChild(langBadge);
                  item.appendChild(confSpan);
                  listContainer.appendChild(item);
                }
                if (table) table.style.display = "block";
              } else {
                if (table) table.style.display = "none";
              }
            } catch (err) {
              console.error("Error fetching ranking:", err);
            }
          }, 150);
        });
      });
    </script>
  </head>
  <body>
    <div class="container">
      <h1>Language Identifier</h1>
      <p class="subtitle">Type or paste text below to detect its language in real time.</p>
      
      <form method="post" action="/demo">
        <div class="textarea-wrapper">
          <textarea name="q" id="typerArea" placeholder="Start typing here..."></textarea>
        </div>
        
        <div id="rankTable" class="results-container">
          <div class="results-title">Detected Languages</div>
          <div id="rankList" class="rank-list">
            <!-- Dynamic elements will be inserted here -->
          </div>
        </div>
        
        <input type="submit" id="manualSubmit" value="Submit Query">
      </form>
    </div>
  </body>
</html>
`
