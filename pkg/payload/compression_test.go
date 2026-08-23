package payload

import (
	"bytes"
	"strings"
	"testing"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	raw := []byte(strings.Repeat("sanitized diff line\n", 100))
	envelope, err := EncodeEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Compression != CompressionZstd {
		t.Fatalf("unexpected compression: %s", envelope.Compression)
	}
	got, err := DecodeEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatal("decompressed payload did not match original")
	}
}
