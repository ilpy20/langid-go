package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ilpy20/langid-go"
)

// ResponseEnvelope represents the shared standard response format for API results.
type ResponseEnvelope struct {
	ResponseData    interface{} `json:"responseData"`
	ResponseDetails *string     `json:"responseDetails"`
	ResponseStatus  int         `json:"responseStatus"`
}

// Server wraps the langid web service.
type Server struct {
	id     *langid.Identifier
	server *http.Server
}

// NewServer creates a new instance of the Server with the provided identifier.
func NewServer(id *langid.Identifier) *Server {
	return &Server{
		id: id,
	}
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
		Addr:    addr,
		Handler: s.NewHandler(),
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

	var responseData map[string]interface{}
	if len(results) > 0 {
		responseData = map[string]interface{}{
			"language":   results[0].Language,
			"confidence": results[0].Score,
		}
	} else {
		responseData = map[string]interface{}{
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

	responseData := make([][2]interface{}, len(results))
	for i, r := range results {
		responseData[i] = [2]interface{}{r.Language, r.Score}
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
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONEnvelope(w, http.StatusBadRequest, nil, pointerToString("failed to read body"))
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
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONEnvelope(w, http.StatusBadRequest, nil, pointerToString("failed to read body"))
			return "", false
		}
		data = string(bodyBytes)

	default:
		writeJSONEnvelope(w, http.StatusMethodNotAllowed, nil, pointerToString(fmt.Sprintf("%s not allowed", r.Method)))
		return "", false
	}
	return data, true
}

func writeJSONEnvelope(w http.ResponseWriter, status int, data interface{}, details *string) {
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

const queryForm = `<html>
  <head>
    <meta http-equiv="Content-type" content="text/html; charset=utf-8">
    <title>Language Identifier</title>
    <script src="//ajax.googleapis.com/ajax/libs/jquery/1.7.2/jquery.min.js" type="text/javascript"></script>
    <script type="text/javascript" charset="utf-8">
      $(document).ready(function() {
        $("#typerArea").keyup(displayType);
      
        function displayType(){
          var contents = $("#typerArea").val();
          if (contents.length != 0) {
            $.post(
              "/rank",
              {q:contents},
              function(data){
                for(i=0;i<5;i++) {
                  $("#lang"+i).html(data.responseData[i][0]);
                  $("#conf"+i).html(data.responseData[i][1]);
                }
                $("#rankTable").show();
              },
              "json"
            );
          }
          else {
            $("#rankTable").hide();
          }
        }
        $("#manualSubmit").remove();
        $("#rankTable").hide();
      });
    </script>
  </head>
  <body>
    <form method=post>
      <center><table>
        <tr>
          <td>
            <textarea name="q" id="typerArea" cols=40 rows=6></textarea></br>
          </td>
        </tr>
        <tr>
          <td>
            <table id="rankTable">
              <tr>
                <td id="lang0">
                  <p>Unable to load jQuery, live update disabled.</p>
                </td><td id="conf0"/>
              </tr>
              <tr><td id="lang1"/><td id="conf1"></tr>
              <tr><td id="lang2"/><td id="conf2"></tr>
              <tr><td id="lang3"/><td id="conf3"></tr>
              <tr><td id="lang4"/><td id="conf4"></tr>
            </table>
            <input type=submit id="manualSubmit" value="submit">
          </td>
        </tr>
      </table></center>
    </form>

  </body>
</html>
`
