package contacts

import (
	"testing"

	ahpcrypto "github.com/stmytsyk/agent-handoff-core/pkg/crypto"
)

func TestBookInitContactStringAndAdd(t *testing.T) {
	identity, err := ahpcrypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	book := Book{Dir: t.TempDir()}
	profile, err := book.Init("alice", "http://relay.local:8799", identity)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Handle != "@alice" {
		t.Fatalf("unexpected handle %q", profile.Handle)
	}
	raw := book.ContactString(profile)
	parsed, err := ParseContactString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Handle != "@alice" || parsed.RelayURL != "http://relay.local:8799" {
		t.Fatalf("unexpected parsed contact: %+v", parsed)
	}
	if _, err := book.Add(parsed); err != nil {
		t.Fatal(err)
	}
	stored, err := book.Get("@alice")
	if err != nil {
		t.Fatal(err)
	}
	if stored.PublicKey != profile.PublicKey {
		t.Fatal("stored public key mismatch")
	}
}
