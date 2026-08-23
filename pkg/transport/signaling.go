package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type PeerRecord struct {
	Handle    string `json:"handle"`
	PublicKey string `json:"public_key"`
	SessionID string `json:"session_id"`
}

type Relay struct {
	mu      sync.Mutex
	peers   map[string]PeerRecord
	inboxes map[string][]Signal
}

func NewRelay() *Relay {
	return &Relay{
		peers:   make(map[string]PeerRecord),
		inboxes: make(map[string][]Signal),
	}
}

func (r *Relay) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", r.register)
	mux.HandleFunc("/signal", r.signal)
	mux.HandleFunc("/poll", r.poll)
	return mux
}

func (r *Relay) register(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var record PeerRecord
	if err := json.NewDecoder(req.Body).Decode(&record); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(record.Handle) == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.peers[record.Handle] = record
	if _, ok := r.inboxes[record.Handle]; !ok {
		r.inboxes[record.Handle] = nil
	}
	r.mu.Unlock()
	writeTransportJSON(w, map[string]any{"ok": true})
}

func (r *Relay) signal(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var signal Signal
	if err := json.NewDecoder(req.Body).Decode(&signal); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(signal.From) == "" || strings.TrimSpace(signal.To) == "" {
		http.Error(w, "from and to are required", http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	_, knownPeer := r.peers[signal.To]
	if knownPeer {
		r.inboxes[signal.To] = append(r.inboxes[signal.To], signal)
	}
	r.mu.Unlock()
	if !knownPeer {
		http.Error(w, "recipient is not registered", http.StatusNotFound)
		return
	}
	writeTransportJSON(w, map[string]any{"ok": true})
}

func (r *Relay) poll(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	handle := req.URL.Query().Get("handle")
	if strings.TrimSpace(handle) == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	signals := append([]Signal(nil), r.inboxes[handle]...)
	r.inboxes[handle] = nil
	r.mu.Unlock()
	writeTransportJSON(w, map[string]any{"signals": signals})
}

type HTTPRelayClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (c HTTPRelayClient) Register(ctx context.Context, record PeerRecord) error {
	var resp struct {
		OK bool `json:"ok"`
	}
	return c.doJSON(ctx, http.MethodPost, "/register", record, &resp)
}

func (c HTTPRelayClient) SendSignal(ctx context.Context, signal Signal) error {
	var resp struct {
		OK bool `json:"ok"`
	}
	return c.doJSON(ctx, http.MethodPost, "/signal", signal, &resp)
}

func (c HTTPRelayClient) Poll(ctx context.Context, handle string) ([]Signal, error) {
	endpoint := "/poll?handle=" + url.QueryEscape(handle)
	var resp struct {
		Signals []Signal `json:"signals"`
	}
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Signals, nil
}

func (c HTTPRelayClient) WaitForSignal(ctx context.Context, handle, signalType string) (Signal, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		signals, err := c.Poll(ctx, handle)
		if err != nil {
			return Signal{}, err
		}
		for _, signal := range signals {
			if signalType == "" || signal.Type == signalType {
				return signal, nil
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return Signal{}, ctx.Err()
		}
	}
}

func (c HTTPRelayClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		return errors.New("relay base URL is required")
	}
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("relay returned %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func writeTransportJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
