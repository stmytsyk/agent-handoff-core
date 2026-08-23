package redaction

import (
	"bytes"
	"math"
	"path/filepath"
	"regexp"
	"strings"
)

var SecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(sk-ant-[a-zA-Z0-9\-_]{32,})`),
	regexp.MustCompile(`(sk-[a-zA-Z0-9]{32,})`),
	regexp.MustCompile(`(ghp_[a-zA-Z0-9]{36})`),
	regexp.MustCompile(`(AKIA[0-9A-Z]{16})`),
}

var SensitiveFilePatterns = []string{
	".env",
	".env.*",
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	"id_rsa",
	"id_ed25519",
}

func CalculateShannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	counts := make(map[rune]float64)
	for _, char := range s {
		counts[char]++
	}
	var entropy float64
	length := float64(len([]rune(s)))
	for _, count := range counts {
		p := count / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func IsSensitivePath(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range SensitiveFilePatterns {
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
	}
	return strings.Contains(path, "/.ssh/") || strings.Contains(path, "\\.ssh\\")
}

func SanitizePayload(input string) string {
	output := input
	output = redactSensitiveDiffFiles(output)
	for _, pattern := range SecretPatterns {
		output = pattern.ReplaceAllString(output, "[REDACTED_API_KEY]")
	}
	words := strings.Fields(output)
	for _, word := range words {
		token := strings.Trim(word, "`'\".,;:()[]{}<>")
		if len(token) >= 16 && CalculateShannonEntropy(token) >= 4.5 {
			output = strings.ReplaceAll(output, token, "[REDACTED_HIGH_ENTROPY_SECRET]")
		}
	}
	return output
}

func SanitizeFile(path string, content []byte) []byte {
	if IsSensitivePath(path) {
		return []byte("[REDACTED_SENSITIVE_FILE]")
	}
	return []byte(SanitizePayload(string(content)))
}

func redactSensitiveDiffFiles(input string) string {
	var out []string
	skip := false
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			skip = false
			if diffLineHasSensitivePath(line) {
				out = append(out, "diff --git [REDACTED_SENSITIVE_FILE]")
				skip = true
				continue
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func diffLineHasSensitivePath(line string) bool {
	for _, field := range strings.Fields(line) {
		path := strings.TrimPrefix(strings.TrimPrefix(field, "a/"), "b/")
		if IsSensitivePath(path) {
			return true
		}
	}
	return false
}

func SanitizeJSONManifest(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return []byte(SanitizePayload(string(data)))
}
