package crypto

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const (
	defaultKeyName = "ahp-ed25519-identity"
)

type Identity struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

type KeyStore interface {
	Load(name string) (Identity, error)
	Store(name string, identity Identity) error
}

type FileKeyStore struct {
	Dir string
}

func DefaultKeyStore() FileKeyStore {
	dir := os.Getenv("AHP_KEYSTORE_DIR")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config", "ahp")
	}
	return FileKeyStore{Dir: dir}
}

func GenerateIdentity() (Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	return Identity{PublicKey: pub, PrivateKey: priv}, nil
}

func LoadOrCreateIdentity(store KeyStore) (Identity, error) {
	identity, err := store.Load(defaultKeyName)
	if err == nil {
		return identity, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Identity{}, err
	}
	identity, err = GenerateIdentity()
	if err != nil {
		return Identity{}, err
	}
	return identity, store.Store(defaultKeyName, identity)
}

func (s FileKeyStore) Load(name string) (Identity, error) {
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		return Identity{}, err
	}
	privBytes, err := hex.DecodeString(string(data))
	if err != nil {
		return Identity{}, err
	}
	if l := len(privBytes); l != ed25519.PrivateKeySize {
		return Identity{}, fmt.Errorf("invalid private key length %d", l)
	}
	priv := ed25519.PrivateKey(privBytes)
	pub := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(pub, priv.Public().(ed25519.PublicKey))
	return Identity{PublicKey: pub, PrivateKey: priv}, nil
}

func (s FileKeyStore) Store(name string, identity Identity) error {
	if len(identity.PrivateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key length %d", len(identity.PrivateKey))
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path(name), []byte(hex.EncodeToString(identity.PrivateKey)), 0o600)
}

func (s FileKeyStore) path(name string) string {
	return filepath.Join(s.Dir, name+".key")
}

func DeriveSAS(a, b ed25519.PublicKey) (string, error) {
	if len(a) != ed25519.PublicKeySize || len(b) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid public key sizes %d and %d", len(a), len(b))
	}
	left := slices.Clone(a)
	right := slices.Clone(b)
	if slices.Compare(left, right) > 0 {
		left, right = right, left
	}
	secret := append(left, right...)
	out, err := hkdf.Key(sha256.New, secret, nil, "ahp sas v1", 8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint64(out)%1_000_000), nil
}
