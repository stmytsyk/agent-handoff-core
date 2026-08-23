package transport

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

type RelayClient interface {
	Register(ctx context.Context, record PeerRecord) error
	SendSignal(ctx context.Context, signal Signal) error
	WaitForSignal(ctx context.Context, handle, signalType string) (Signal, error)
}

type WebRTCOptions struct {
	LocalHandle  string
	RemoteHandle string
	PublicKey    string
	SessionID    string
	ICEServers   []webrtc.ICEServer
}

type WebRTCChannel struct {
	pc       *webrtc.PeerConnection
	dc       *webrtc.DataChannel
	inbound  chan []byte
	openOnce sync.Once
	open     chan struct{}
}

func DialWebRTC(ctx context.Context, relay RelayClient, opts WebRTCOptions) (*WebRTCChannel, error) {
	if err := validateWebRTCOptions(opts); err != nil {
		return nil, err
	}
	if err := relay.Register(ctx, PeerRecord{Handle: opts.LocalHandle, PublicKey: opts.PublicKey, SessionID: opts.SessionID}); err != nil {
		return nil, err
	}
	channel, err := newWebRTCChannel(opts.ICEServers)
	if err != nil {
		return nil, err
	}
	dc, err := channel.pc.CreateDataChannel("ahp", nil)
	if err != nil {
		channel.Close()
		return nil, err
	}
	channel.attachDataChannel(dc)

	offer, err := channel.pc.CreateOffer(nil)
	if err != nil {
		channel.Close()
		return nil, err
	}
	gatherComplete := webrtc.GatheringCompletePromise(channel.pc)
	if err := channel.pc.SetLocalDescription(offer); err != nil {
		channel.Close()
		return nil, err
	}
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		channel.Close()
		return nil, ctx.Err()
	}
	if err := sendSessionDescription(ctx, relay, opts.LocalHandle, opts.RemoteHandle, "offer", *channel.pc.LocalDescription()); err != nil {
		channel.Close()
		return nil, err
	}
	answer, err := waitSessionDescription(ctx, relay, opts.LocalHandle, "answer")
	if err != nil {
		channel.Close()
		return nil, err
	}
	if err := channel.pc.SetRemoteDescription(answer); err != nil {
		channel.Close()
		return nil, err
	}
	if err := channel.waitOpen(ctx); err != nil {
		channel.Close()
		return nil, err
	}
	return channel, nil
}

func AcceptWebRTC(ctx context.Context, relay RelayClient, opts WebRTCOptions) (*WebRTCChannel, error) {
	if err := validateWebRTCOptions(opts); err != nil {
		return nil, err
	}
	if err := relay.Register(ctx, PeerRecord{Handle: opts.LocalHandle, PublicKey: opts.PublicKey, SessionID: opts.SessionID}); err != nil {
		return nil, err
	}
	channel, err := newWebRTCChannel(opts.ICEServers)
	if err != nil {
		return nil, err
	}
	channel.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "ahp" {
			channel.attachDataChannel(dc)
		}
	})
	offer, err := waitSessionDescription(ctx, relay, opts.LocalHandle, "offer")
	if err != nil {
		channel.Close()
		return nil, err
	}
	if err := channel.pc.SetRemoteDescription(offer); err != nil {
		channel.Close()
		return nil, err
	}
	answer, err := channel.pc.CreateAnswer(nil)
	if err != nil {
		channel.Close()
		return nil, err
	}
	gatherComplete := webrtc.GatheringCompletePromise(channel.pc)
	if err := channel.pc.SetLocalDescription(answer); err != nil {
		channel.Close()
		return nil, err
	}
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		channel.Close()
		return nil, ctx.Err()
	}
	if err := sendSessionDescription(ctx, relay, opts.LocalHandle, opts.RemoteHandle, "answer", *channel.pc.LocalDescription()); err != nil {
		channel.Close()
		return nil, err
	}
	if err := channel.waitOpen(ctx); err != nil {
		channel.Close()
		return nil, err
	}
	return channel, nil
}

func newWebRTCChannel(iceServers []webrtc.ICEServer) (*WebRTCChannel, error) {
	var settings webrtc.SettingEngine
	settings.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settings))
	pc, err := api.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
	if err != nil {
		return nil, err
	}
	return &WebRTCChannel{
		pc:      pc,
		inbound: make(chan []byte, 16),
		open:    make(chan struct{}),
	}, nil
}

func (c *WebRTCChannel) attachDataChannel(dc *webrtc.DataChannel) {
	c.dc = dc
	dc.OnOpen(func() {
		c.openOnce.Do(func() {
			close(c.open)
		})
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			c.inbound <- []byte(msg.Data)
			return
		}
		c.inbound <- append([]byte(nil), msg.Data...)
	})
}

func (c *WebRTCChannel) Send(ctx context.Context, payload []byte) error {
	if err := c.waitOpen(ctx); err != nil {
		return err
	}
	return c.dc.Send(payload)
}

func (c *WebRTCChannel) Flush(ctx context.Context) error {
	if err := c.waitOpen(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if c.dc.BufferedAmount() == 0 {
			return nil
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *WebRTCChannel) Receive(ctx context.Context) ([]byte, error) {
	select {
	case payload := <-c.inbound:
		return payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *WebRTCChannel) Close() error {
	if c == nil || c.pc == nil {
		return nil
	}
	return c.pc.Close()
}

func (c *WebRTCChannel) waitOpen(ctx context.Context) error {
	if c == nil {
		return errors.New("webrtc channel is nil")
	}
	select {
	case <-c.open:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sendSessionDescription(ctx context.Context, relay RelayClient, from, to, signalType string, desc webrtc.SessionDescription) error {
	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	return relay.SendSignal(ctx, Signal{From: from, To: to, Type: signalType, Data: data})
}

func waitSessionDescription(ctx context.Context, relay RelayClient, handle, signalType string) (webrtc.SessionDescription, error) {
	signal, err := relay.WaitForSignal(ctx, handle, signalType)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	var desc webrtc.SessionDescription
	if err := json.Unmarshal(signal.Data, &desc); err != nil {
		return webrtc.SessionDescription{}, err
	}
	return desc, nil
}

func validateWebRTCOptions(opts WebRTCOptions) error {
	if opts.LocalHandle == "" {
		return errors.New("local handle is required")
	}
	if opts.RemoteHandle == "" {
		return errors.New("remote handle is required")
	}
	return nil
}
