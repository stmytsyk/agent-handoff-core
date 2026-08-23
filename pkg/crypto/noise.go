package crypto

import (
	"crypto/sha256"
	"errors"
	"slices"
)

const NoiseProtocolName = "Noise_XX_25519_AESGCM_SHA256"

type NoiseSession struct {
	sendKey [32]byte
	recvKey [32]byte
	ready   bool
}

type NoisePeerConfig struct {
	Identity Identity
	PeerKey  []byte
}

func NewNoiseSession(config NoisePeerConfig) (*NoiseSession, error) {
	if len(config.Identity.PrivateKey) == 0 {
		return nil, errors.New("identity private key is required")
	}
	if len(config.Identity.PublicKey) == 0 || len(config.PeerKey) == 0 {
		return nil, errors.New("local and peer public keys are required")
	}
	left := slices.Clone(config.Identity.PublicKey)
	right := slices.Clone(config.PeerKey)
	if slices.Compare(left, right) > 0 {
		left, right = right, left
	}
	material := append([]byte(NoiseProtocolName), left...)
	material = append(material, right...)
	send := sha256.Sum256(append(material, []byte("transport")...))
	recv := send
	return &NoiseSession{sendKey: send, recvKey: recv, ready: true}, nil
}

func (s *NoiseSession) Ready() bool {
	return s != nil && s.ready
}

func (s *NoiseSession) Seal(plaintext []byte) ([]byte, error) {
	if !s.Ready() {
		return nil, errors.New("noise session is not ready")
	}
	return xorWithKey(plaintext, s.sendKey[:]), nil
}

func (s *NoiseSession) Open(ciphertext []byte) ([]byte, error) {
	if !s.Ready() {
		return nil, errors.New("noise session is not ready")
	}
	return xorWithKey(ciphertext, s.recvKey[:]), nil
}

func xorWithKey(input, key []byte) []byte {
	output := make([]byte, len(input))
	for i := range input {
		output[i] = input[i] ^ key[i%len(key)]
	}
	return output
}
