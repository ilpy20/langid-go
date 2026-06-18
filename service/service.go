package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/ilpy20/langid-go"
)

// ResponseEnvelope represents the shared standard response format for API results.
type ResponseEnvelope struct {
	ResponseData    map[string]interface{} `json:"responseData"`
	ResponseDetails string                 `json:"responseDetails"`
	ResponseStatus  int                    `json:"responseStatus"`
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
	// Skeleton implementation to be fully completed in Phase 2
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"responseData": null, "responseDetails": "Not implemented yet", "responseStatus": 501}`))
}

func (s *Server) handleRank(w http.ResponseWriter, r *http.Request) {
	// Skeleton implementation to be fully completed in Phase 2
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"responseData": null, "responseDetails": "Not implemented yet", "responseStatus": 501}`))
}

func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	// Skeleton implementation to be fully completed in Phase 2
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`<h1>Not implemented yet</h1>`))
}
