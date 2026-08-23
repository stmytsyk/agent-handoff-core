package tests

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"

	ahpcrypto "github.com/agent-handoff-protocol/ahp-core/pkg/crypto"
	"github.com/agent-handoff-protocol/ahp-core/pkg/payload"
	"github.com/agent-handoff-protocol/ahp-core/pkg/redaction"
	"github.com/agent-handoff-protocol/ahp-core/pkg/transport"
)

func TestP2PHandoffSanitizesAndTransfers(t *testing.T) {
	input := "token sk-ant-abcdefghijklmnopqrstuvwxyz0123456789 and aws AKIAABCDEFGHIJKLMNOP"
	sanitized := redaction.SanitizePayload(input)
	if sanitized == input {
		t.Fatal("expected sanitizer to redact mock API keys")
	}

	a, err := ahpcrypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	b, err := ahpcrypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	sasAB, err := ahpcrypto.DeriveSAS(a.PublicKey, b.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	sasBA, err := ahpcrypto.DeriveSAS(b.PublicKey, a.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if sasAB != sasBA || len(sasAB) != 6 {
		t.Fatalf("unexpected SAS values %q and %q", sasAB, sasBA)
	}
	envelope, err := payload.EncodeEnvelope([]byte(sanitized))
	if err != nil {
		t.Fatal(err)
	}

	left, right := transport.NewMemoryChannelPair()
	sessionA, err := ahpcrypto.NewNoiseSession(ahpcrypto.NoisePeerConfig{Identity: a, PeerKey: b.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err := ahpcrypto.NewNoiseSession(ahpcrypto.NoisePeerConfig{Identity: IdentityForTest(b.PrivateKey), PeerKey: a.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	secureA, err := transport.WrapWithNoise(left, sessionA)
	if err != nil {
		t.Fatal(err)
	}
	secureB, err := transport.WrapWithNoise(right, sessionB)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	wirePayload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := secureA.Send(ctx, wirePayload); err != nil {
		t.Fatal(err)
	}
	received, err := secureB.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var receivedEnvelope payload.Envelope
	if err := json.Unmarshal(received, &receivedEnvelope); err != nil {
		t.Fatal(err)
	}
	decompressed, err := payload.DecodeEnvelope(receivedEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if string(decompressed) != sanitized {
		t.Fatalf("received mismatch\nwant: %s\n got: %s", sanitized, decompressed)
	}
}

func IdentityForTest(priv ed25519.PrivateKey) ahpcrypto.Identity {
	return ahpcrypto.Identity{PrivateKey: priv, PublicKey: priv.Public().(ed25519.PublicKey)}
}
