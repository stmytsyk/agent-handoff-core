package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type KeychainStore struct {
	Service string
	Account string
}

func DefaultKeychainStore() KeychainStore {
	return KeychainStore{Service: "ahp-core", Account: defaultKeyName}
}

func (s KeychainStore) Load(name string) (Identity, error) {
	if runtime.GOOS != "darwin" {
		return Identity{}, errors.New("os keychain adapter is only implemented for darwin")
	}
	account := s.account(name)
	out, err := exec.Command("security", "find-generic-password", "-s", s.service(), "-a", account, "-w").Output()
	if err != nil {
		return Identity{}, err
	}
	privBytes, err := hex.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return Identity{}, err
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return Identity{}, fmt.Errorf("invalid keychain private key length %d", len(privBytes))
	}
	priv := ed25519.PrivateKey(privBytes)
	return Identity{PrivateKey: priv, PublicKey: priv.Public().(ed25519.PublicKey)}, nil
}

func (s KeychainStore) Store(name string, identity Identity) error {
	if runtime.GOOS != "darwin" {
		return errors.New("os keychain adapter is only implemented for darwin")
	}
	if len(identity.PrivateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key length %d", len(identity.PrivateKey))
	}
	account := s.account(name)
	secret := hex.EncodeToString(identity.PrivateKey)
	_ = exec.Command("security", "delete-generic-password", "-s", s.service(), "-a", account).Run()
	return exec.Command("security", "add-generic-password", "-s", s.service(), "-a", account, "-w", secret, "-U").Run()
}

func (s KeychainStore) service() string {
	if s.Service != "" {
		return s.Service
	}
	return "ahp-core"
}

func (s KeychainStore) account(name string) string {
	if name != "" {
		return name
	}
	if s.Account != "" {
		return s.Account
	}
	return defaultKeyName
}
