GO ?= go
GOFILES := $(shell find . -name '*.go' -not -path './.git/*')

.PHONY: all fmt fmt-check vet test build vulncheck check webrtc-test

all: check

fmt:
	gofmt -w $(GOFILES)

fmt-check:
	test -z "$$(gofmt -l $(GOFILES))"

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	$(GO) build ./cmd/ahpd ./cmd/ahp-cli ./cmd/ahp-relay

vulncheck:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: fmt-check vet test build vulncheck

webrtc-test:
	AHP_WEBRTC_TEST=1 $(GO) test ./cmd/ahpd ./pkg/transport -run 'TestSendReceiveEnvelopeViaRelayWebRTC|TestWebRTCChannelViaRelay' -count=1 -v
