package main

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stmytsyk/agent-handoff-core/pkg/contacts"
	ahpcrypto "github.com/stmytsyk/agent-handoff-core/pkg/crypto"
	"github.com/stmytsyk/agent-handoff-core/pkg/payload"
	"github.com/stmytsyk/agent-handoff-core/pkg/transport"
)

func TestSendReceiveEnvelopeViaRelayWebRTC(t *testing.T) {
	if os.Getenv("AHP_WEBRTC_TEST") != "1" {
		t.Skip("set AHP_WEBRTC_TEST=1 to run daemon WebRTC integration test")
	}
	tempDir := t.TempDir()
	t.Setenv("AHP_CONFIG_DIR", tempDir)
	t.Setenv("AHP_KEYSTORE_DIR", tempDir)
	handler := transport.NewRelay().Handler()
	server := httptest.NewServer(handler)
	defer server.Close()

	envelope, err := payload.EncodeEnvelope([]byte("daemon handoff payload"))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := ahpcrypto.LoadOrCreateIdentity(ahpcrypto.DefaultKeyStore())
	if err != nil {
		t.Fatal(err)
	}
	book := contacts.DefaultBook()
	if _, err := book.Init("@alice", server.URL, identity); err != nil {
		t.Fatal(err)
	}
	publicKey := contacts.EncodePublicKey(identity.PublicKey)
	if _, err := book.Add(contacts.Contact{Handle: "@alice", PublicKey: publicKey, RelayURL: server.URL, Trusted: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := book.Add(contacts.Contact{Handle: "@bob", PublicKey: publicKey, RelayURL: server.URL, Trusted: true}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	receivedCh := make(chan payload.Envelope, 1)
	errCh := make(chan error, 1)
	go func() {
		received, err := receiveEnvelope(ctx, server.URL, "@bob", "@alice", 10_000)
		if err != nil {
			errCh <- err
			return
		}
		receivedCh <- received
	}()

	if err := sendEnvelope(ctx, server.URL, "@alice", "@bob", 10_000, envelope); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errCh:
		t.Fatal(err)
	case received := <-receivedCh:
		raw, err := payload.DecodeEnvelope(received)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "daemon handoff payload" {
			t.Fatalf("unexpected payload: %q", raw)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
