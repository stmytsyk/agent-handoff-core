package transport

import (
	"context"
	"errors"
	"sync"

	ahpcrypto "github.com/stmytsyk/agent-handoff-core/pkg/crypto"
)

type Signal struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type Signaler interface {
	Exchange(ctx context.Context, signal Signal) (Signal, error)
}

type DataChannel interface {
	Send(ctx context.Context, payload []byte) error
	Receive(ctx context.Context) ([]byte, error)
}

type MemoryChannel struct {
	mu      sync.Mutex
	closed  bool
	inbound chan []byte
	peer    *MemoryChannel
}

func NewMemoryChannelPair() (*MemoryChannel, *MemoryChannel) {
	a := &MemoryChannel{inbound: make(chan []byte, 16)}
	b := &MemoryChannel{inbound: make(chan []byte, 16)}
	a.peer = b
	b.peer = a
	return a, b
}

func (c *MemoryChannel) Send(ctx context.Context, payload []byte) error {
	c.mu.Lock()
	closed := c.closed
	peer := c.peer
	c.mu.Unlock()
	if closed || peer == nil {
		return errors.New("data channel is closed")
	}
	msg := append([]byte(nil), payload...)
	select {
	case peer.inbound <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *MemoryChannel) Receive(ctx context.Context) ([]byte, error) {
	select {
	case payload := <-c.inbound:
		return payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type SecureChannel struct {
	channel DataChannel
	session *ahpcrypto.NoiseSession
}

func WrapWithNoise(channel DataChannel, session *ahpcrypto.NoiseSession) (*SecureChannel, error) {
	if channel == nil {
		return nil, errors.New("data channel is required")
	}
	if !session.Ready() {
		return nil, errors.New("noise session is not ready")
	}
	return &SecureChannel{channel: channel, session: session}, nil
}

func (c *SecureChannel) Send(ctx context.Context, payload []byte) error {
	sealed, err := c.session.Seal(payload)
	if err != nil {
		return err
	}
	return c.channel.Send(ctx, sealed)
}

func (c *SecureChannel) Receive(ctx context.Context) ([]byte, error) {
	sealed, err := c.channel.Receive(ctx)
	if err != nil {
		return nil, err
	}
	return c.session.Open(sealed)
}
