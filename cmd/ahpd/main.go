package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agent-handoff-protocol/ahp-core/pkg/contacts"
	ahpcrypto "github.com/agent-handoff-protocol/ahp-core/pkg/crypto"
	"github.com/agent-handoff-protocol/ahp-core/pkg/payload"
	"github.com/agent-handoff-protocol/ahp-core/pkg/transport"
)

const socketPath = "/tmp/ahp.sock"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Printf("ahpd listening on unix://%s", socketPath)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("accept: %v", err)
			continue
		}
		go handleConn(ctx, conn)
	}
}

func handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	var req struct {
		Command       string   `json:"command"`
		Target        string   `json:"target"`
		LocalHandle   string   `json:"local_handle"`
		RelayURL      string   `json:"relay_url"`
		TimeoutMS     int64    `json:"timeout_ms"`
		ContactString string   `json:"contact_string"`
		Trusted       bool     `json:"trusted"`
		Summary       []string `json:"summary"`
		PendingTasks  []string `json:"pending_tasks"`
	}
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	switch req.Command {
	case "init":
		profile, err := initProfile(req.LocalHandle, req.RelayURL)
		if err != nil {
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "profile": profile, "contact": contacts.DefaultBook().ContactString(profile)})
	case "contact_show":
		profile, err := contacts.DefaultBook().LocalProfile()
		if err != nil {
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "profile": profile, "contact": contacts.DefaultBook().ContactString(profile)})
	case "contact_add":
		contact, err := contacts.DefaultBook().AddFromString(req.ContactString)
		if err != nil {
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if req.Trusted {
			contact.Trusted = true
			contact, err = contacts.DefaultBook().Add(contact)
			if err != nil {
				_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
		sas := ""
		if local, err := ahpcrypto.LoadOrCreateIdentity(ahpcrypto.DefaultKeyStore()); err == nil {
			if peer, err := contacts.DecodePublicKey(contact.PublicKey); err == nil {
				sas, _ = ahpcrypto.DeriveSAS(local.PublicKey, peer)
			}
		}
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "contact": contact, "sas": sas})
	case "share":
		envelope, err := (payload.Builder{}).CompressedEnvelope(ctx, payload.Options{Summary: req.Summary, PendingTasks: req.PendingTasks})
		if err != nil {
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if shouldUseRelay(req.RelayURL, req.Target) {
			if err := sendEnvelope(ctx, req.RelayURL, req.LocalHandle, req.Target, req.TimeoutMS, envelope); err != nil {
				_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
				return
			}
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "delivered": true, "target": req.Target})
			return
		}
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "envelope": envelope})
	case "ingest":
		if req.RelayURL == "" || req.Target == "" {
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "accepted": true})
			return
		}
		envelope, err := receiveEnvelope(ctx, req.RelayURL, req.LocalHandle, req.Target, req.TimeoutMS)
		if err != nil {
			_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "accepted": true, "from": req.Target, "envelope": envelope})
	default:
		_ = json.NewEncoder(conn).Encode(map[string]any{"ok": false, "error": "unknown command"})
	}
}

func sendEnvelope(ctx context.Context, relayURL, localHandle, remoteHandle string, timeoutMS int64, envelope payload.Envelope) error {
	ctx, cancel := withRequestTimeout(ctx, timeoutMS, 30*time.Second)
	defer cancel()
	if remoteHandle == "" {
		return fmt.Errorf("target handle is required when using relay")
	}
	route, err := resolveShareRoute(relayURL, localHandle, remoteHandle)
	if err != nil {
		return err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	client := transport.HTTPRelayClient{BaseURL: route.RelayURL}
	channel, err := transport.DialWebRTC(ctx, client, transport.WebRTCOptions{
		LocalHandle:  route.LocalHandle,
		RemoteHandle: route.RemoteHandle,
		PublicKey:    route.LocalPublicKey,
		SessionID:    fmt.Sprintf("%s-%d", route.LocalHandle, time.Now().UnixNano()),
	})
	if err != nil {
		return err
	}
	defer channel.Close()
	secure, err := secureWebRTCChannel(channel, route)
	if err != nil {
		return err
	}
	if err := secure.Send(ctx, data); err != nil {
		return err
	}
	return channel.Flush(ctx)
}

func receiveEnvelope(ctx context.Context, relayURL, localHandle, remoteHandle string, timeoutMS int64) (payload.Envelope, error) {
	ctx, cancel := withRequestTimeout(ctx, timeoutMS, 60*time.Second)
	defer cancel()
	if remoteHandle == "" {
		return payload.Envelope{}, fmt.Errorf("peer handle is required when using relay")
	}
	route, err := resolveShareRoute(relayURL, localHandle, remoteHandle)
	if err != nil {
		return payload.Envelope{}, err
	}
	client := transport.HTTPRelayClient{BaseURL: route.RelayURL}
	channel, err := transport.AcceptWebRTC(ctx, client, transport.WebRTCOptions{
		LocalHandle:  route.LocalHandle,
		RemoteHandle: route.RemoteHandle,
		PublicKey:    route.LocalPublicKey,
		SessionID:    fmt.Sprintf("%s-%d", route.LocalHandle, time.Now().UnixNano()),
	})
	if err != nil {
		return payload.Envelope{}, err
	}
	defer channel.Close()
	secure, err := secureWebRTCChannel(channel, route)
	if err != nil {
		return payload.Envelope{}, err
	}
	data, err := secure.Receive(ctx)
	if err != nil {
		return payload.Envelope{}, err
	}
	var envelope payload.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return payload.Envelope{}, err
	}
	if _, err := payload.DecodeEnvelope(envelope); err != nil {
		return payload.Envelope{}, err
	}
	return envelope, nil
}

func initProfile(handle, relayURL string) (contacts.LocalProfile, error) {
	identity, err := ahpcrypto.LoadOrCreateIdentity(ahpcrypto.DefaultKeyStore())
	if err != nil {
		return contacts.LocalProfile{}, err
	}
	return contacts.DefaultBook().Init(handle, relayURL, identity)
}

type shareRoute struct {
	RelayURL       string
	LocalHandle    string
	RemoteHandle   string
	LocalPublicKey string
	Identity       ahpcrypto.Identity
	PeerPublicKey  []byte
}

func resolveShareRoute(relayURL, localHandle, remoteHandle string) (shareRoute, error) {
	book := contacts.DefaultBook()
	profile, err := book.LocalProfile()
	if err == nil {
		if localHandle == "" {
			localHandle = profile.Handle
		}
		if relayURL == "" {
			relayURL = profile.RelayURL
		}
	}
	identity, err := ahpcrypto.LoadOrCreateIdentity(ahpcrypto.DefaultKeyStore())
	if err != nil {
		return shareRoute{}, err
	}
	localPublicKey := contacts.EncodePublicKey(identity.PublicKey)
	contact, err := book.Get(remoteHandle)
	if err != nil {
		return shareRoute{}, fmt.Errorf("contact %s is not known; run ahp contact add <contact-string>", contacts.NormalizeHandle(remoteHandle))
	}
	if !contact.Trusted {
		return shareRoute{}, fmt.Errorf("contact %s is not trusted; verify SAS and re-add with --trust", contact.Handle)
	}
	peerPublicKey, err := contacts.DecodePublicKey(contact.PublicKey)
	if err != nil {
		return shareRoute{}, err
	}
	remoteHandle = contact.Handle
	if relayURL == "" {
		relayURL = contact.RelayURL
	}
	if localHandle == "" {
		return shareRoute{}, fmt.Errorf("local handle is required; run ahp init --handle @you")
	}
	if remoteHandle == "" {
		return shareRoute{}, fmt.Errorf("remote handle is required")
	}
	if relayURL == "" {
		return shareRoute{}, fmt.Errorf("relay URL is required")
	}
	return shareRoute{
		RelayURL:       relayURL,
		LocalHandle:    contacts.NormalizeHandle(localHandle),
		RemoteHandle:   contacts.NormalizeHandle(remoteHandle),
		LocalPublicKey: localPublicKey,
		Identity:       identity,
		PeerPublicKey:  peerPublicKey,
	}, nil
}

func shouldUseRelay(relayURL, target string) bool {
	if relayURL != "" {
		return true
	}
	if target == "" {
		return false
	}
	contact, err := contacts.DefaultBook().Get(target)
	return err == nil && contact.RelayURL != ""
}

func secureWebRTCChannel(channel transport.DataChannel, route shareRoute) (*transport.SecureChannel, error) {
	session, err := ahpcrypto.NewNoiseSession(ahpcrypto.NoisePeerConfig{
		Identity: route.Identity,
		PeerKey:  route.PeerPublicKey,
	})
	if err != nil {
		return nil, err
	}
	return transport.WrapWithNoise(channel, session)
}

func withRequestTimeout(parent context.Context, timeoutMS int64, fallback time.Duration) (context.Context, context.CancelFunc) {
	timeout := fallback
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	return context.WithTimeout(parent, timeout)
}
