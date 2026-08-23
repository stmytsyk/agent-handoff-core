package redaction

import (
	"strings"
	"testing"
)

func TestSanitizePayloadRedactsRegexAndEntropy(t *testing.T) {
	input := "api sk-ant-abcdefghijklmnopqrstuvwxyz0123456789 ghp_abcdefghijklmnopqrstuvwxyzABCDEFGHIJ random abcdefghijklmnopqrstuvwxyz012345"
	got := SanitizePayload(input)
	if strings.Contains(got, "sk-ant-") || strings.Contains(got, "ghp_") {
		t.Fatalf("secret pattern leaked: %s", got)
	}
	if !strings.Contains(got, "[REDACTED_HIGH_ENTROPY_SECRET]") {
		t.Fatalf("high entropy token was not redacted: %s", got)
	}
}

func TestSanitizeFileBlocksSensitivePaths(t *testing.T) {
	got := string(SanitizeFile(".env", []byte("TOKEN=sk-ant-abcdefghijklmnopqrstuvwxyz0123456789")))
	if got != "[REDACTED_SENSITIVE_FILE]" {
		t.Fatalf("unexpected sensitive file output: %s", got)
	}
}
