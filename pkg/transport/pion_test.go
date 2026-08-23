package transport

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestWebRTCChannelViaRelay(t *testing.T) {
	if os.Getenv("AHP_WEBRTC_TEST") != "1" {
		t.Skip("set AHP_WEBRTC_TEST=1 to run WebRTC ICE/DataChannel integration test")
	}
	handler := NewRelay().Handler()
	relay := HTTPRelayClient{
		BaseURL:    "http://ahp-relay.test",
		HTTPClient: &http.Client{Transport: handlerRoundTripper{handler: handler}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var answerer *WebRTCChannel
	errCh := make(chan error, 1)
	go func() {
		var err error
		answerer, err = AcceptWebRTC(ctx, relay, WebRTCOptions{
			LocalHandle:  "@bob",
			RemoteHandle: "@alice",
			PublicKey:    "ed25519:bob",
			SessionID:    "bob-1",
		})
		errCh <- err
	}()

	offerer, err := DialWebRTC(ctx, relay, WebRTCOptions{
		LocalHandle:  "@alice",
		RemoteHandle: "@bob",
		PublicKey:    "ed25519:alice",
		SessionID:    "alice-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer offerer.Close()

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	defer answerer.Close()

	if err := offerer.Send(ctx, []byte("hello over webrtc")); err != nil {
		t.Fatal(err)
	}
	got, err := answerer.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello over webrtc" {
		t.Fatalf("unexpected payload: %q", got)
	}
}
