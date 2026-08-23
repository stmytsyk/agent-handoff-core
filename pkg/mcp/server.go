package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/agent-handoff-protocol/ahp-core/pkg/payload"
)

type Server struct {
	builder payload.Builder
}

func NewServer(builder payload.Builder) *Server {
	return &Server{builder: builder}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/share", s.share)
	mux.HandleFunc("/ingest", s.ingest)
	mux.HandleFunc("/agent_ask", s.agentAsk)
	mux.HandleFunc("/tools", s.tools)
	return mux
}

func (s *Server) share(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Summary      []string `json:"summary"`
		PendingTasks []string `json:"pending_tasks"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	envelope, err := s.builder.CompressedEnvelope(r.Context(), payload.Options{Summary: req.Summary, PendingTasks: req.PendingTasks})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"tool": "share_context", "envelope": envelope})
}

func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var manifest payload.Manifest
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"tool": "ingest_context", "schema_version": manifest.SchemaVersion, "accepted": true})
}

func (s *Server) agentAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{"tool": "agent_ask", "status": "queued"})
}

func (s *Server) tools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"tools": []string{"share_context", "ingest_context", "agent_ask"},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func ListenAndServe(ctx context.Context, addr string, builder payload.Builder) error {
	server := &http.Server{Addr: addr, Handler: NewServer(builder).Handler()}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	return server.ListenAndServe()
}
