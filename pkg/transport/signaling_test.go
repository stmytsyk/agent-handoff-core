package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPRelayExchangesSignals(t *testing.T) {
	handler := NewRelay().Handler()
	client := HTTPRelayClient{
		BaseURL:    "http://ahp-relay.test",
		HTTPClient: &http.Client{Transport: handlerRoundTripper{handler: handler}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Register(ctx, PeerRecord{Handle: "@alice", PublicKey: "ed25519:alice", SessionID: "alice-1"}); err != nil {
		t.Fatal(err)
	}
	if err := client.Register(ctx, PeerRecord{Handle: "@bob", PublicKey: "ed25519:bob", SessionID: "bob-1"}); err != nil {
		t.Fatal(err)
	}
	if err := client.SendSignal(ctx, Signal{From: "@alice", To: "@bob", Type: "offer", Data: []byte("sdp-offer")}); err != nil {
		t.Fatal(err)
	}
	offer, err := client.WaitForSignal(ctx, "@bob", "offer")
	if err != nil {
		t.Fatal(err)
	}
	if offer.From != "@alice" || string(offer.Data) != "sdp-offer" {
		t.Fatalf("unexpected offer: %+v", offer)
	}
	if err := client.SendSignal(ctx, Signal{From: "@bob", To: "@alice", Type: "answer", Data: []byte("sdp-answer")}); err != nil {
		t.Fatal(err)
	}
	answer, err := client.WaitForSignal(ctx, "@alice", "answer")
	if err != nil {
		t.Fatal(err)
	}
	if answer.From != "@bob" || string(answer.Data) != "sdp-answer" {
		t.Fatalf("unexpected answer: %+v", answer)
	}
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (h handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	h.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}
