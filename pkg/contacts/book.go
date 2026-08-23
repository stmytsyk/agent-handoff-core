package contacts

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ahpcrypto "github.com/agent-handoff-protocol/ahp-core/pkg/crypto"
)

type LocalProfile struct {
	Handle    string `json:"handle"`
	PublicKey string `json:"public_key"`
	RelayURL  string `json:"relay_url,omitempty"`
}

type Contact struct {
	Handle      string    `json:"handle"`
	PublicKey   string    `json:"public_key"`
	RelayURL    string    `json:"relay_url,omitempty"`
	Trusted     bool      `json:"trusted"`
	AddedAt     time.Time `json:"added_at"`
	VerifiedSAS string    `json:"verified_sas,omitempty"`
}

type Book struct {
	Dir string
}

func DefaultBook() Book {
	dir := os.Getenv("AHP_CONFIG_DIR")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config", "ahp")
	}
	return Book{Dir: dir}
}

func (b Book) Init(handle, relayURL string, identity ahpcrypto.Identity) (LocalProfile, error) {
	handle = NormalizeHandle(handle)
	if handle == "" {
		return LocalProfile{}, errors.New("handle is required")
	}
	if len(identity.PublicKey) != ed25519.PublicKeySize {
		return LocalProfile{}, fmt.Errorf("invalid local public key length %d", len(identity.PublicKey))
	}
	profile := LocalProfile{
		Handle:    handle,
		PublicKey: EncodePublicKey(identity.PublicKey),
		RelayURL:  relayURL,
	}
	if err := os.MkdirAll(b.Dir, 0o700); err != nil {
		return LocalProfile{}, err
	}
	return profile, b.writeJSON("identity.json", profile, 0o600)
}

func (b Book) LocalProfile() (LocalProfile, error) {
	var profile LocalProfile
	if err := b.readJSON("identity.json", &profile); err != nil {
		return LocalProfile{}, err
	}
	return profile, nil
}

func (b Book) Add(contact Contact) (Contact, error) {
	contact.Handle = NormalizeHandle(contact.Handle)
	if contact.Handle == "" {
		return Contact{}, errors.New("contact handle is required")
	}
	if _, err := DecodePublicKey(contact.PublicKey); err != nil {
		return Contact{}, err
	}
	contacts, err := b.All()
	if err != nil {
		return Contact{}, err
	}
	if contact.AddedAt.IsZero() {
		contact.AddedAt = time.Now().UTC()
	}
	contacts[contact.Handle] = contact
	return contact, b.writeContacts(contacts)
}

func (b Book) AddFromString(raw string) (Contact, error) {
	contact, err := ParseContactString(raw)
	if err != nil {
		return Contact{}, err
	}
	return b.Add(contact)
}

func (b Book) Get(handle string) (Contact, error) {
	contacts, err := b.All()
	if err != nil {
		return Contact{}, err
	}
	contact, ok := contacts[NormalizeHandle(handle)]
	if !ok {
		return Contact{}, os.ErrNotExist
	}
	return contact, nil
}

func (b Book) All() (map[string]Contact, error) {
	contacts := make(map[string]Contact)
	err := b.readJSON("contacts.json", &contacts)
	if errors.Is(err, os.ErrNotExist) {
		return contacts, nil
	}
	return contacts, err
}

func (b Book) ContactString(profile LocalProfile) string {
	return FormatContactString(profile.Handle, profile.RelayURL, profile.PublicKey)
}

func FormatContactString(handle, relayURL, publicKey string) string {
	handle = NormalizeHandle(handle)
	relayURL = strings.TrimSpace(relayURL)
	if relayURL == "" {
		return fmt.Sprintf("ahp:%s#%s", strings.TrimPrefix(handle, "@"), publicKey)
	}
	return fmt.Sprintf("ahp:%s@%s#%s", strings.TrimPrefix(handle, "@"), relayURL, publicKey)
}

func ParseContactString(raw string) (Contact, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "ahp:") {
		return Contact{}, errors.New("contact string must start with ahp:")
	}
	body := strings.TrimPrefix(raw, "ahp:")
	parts := strings.SplitN(body, "#", 2)
	if len(parts) != 2 {
		return Contact{}, errors.New("contact string must include #ed25519:<hex>")
	}
	publicKey := parts[1]
	if _, err := DecodePublicKey(publicKey); err != nil {
		return Contact{}, err
	}
	handlePart := parts[0]
	var handle, relayURL string
	if at := strings.Index(handlePart, "@"); at >= 0 {
		handle = handlePart[:at]
		relayURL = handlePart[at+1:]
	} else {
		handle = handlePart
	}
	return Contact{
		Handle:    NormalizeHandle(handle),
		PublicKey: publicKey,
		RelayURL:  relayURL,
		Trusted:   false,
		AddedAt:   time.Now().UTC(),
	}, nil
}

func EncodePublicKey(key ed25519.PublicKey) string {
	return "ed25519:" + hex.EncodeToString(key)
}

func DecodePublicKey(raw string) (ed25519.PublicKey, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "ed25519:"))
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key length %d", len(key))
	}
	return ed25519.PublicKey(key), nil
}

func NormalizeHandle(handle string) string {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return ""
	}
	if !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}
	return handle
}

func (b Book) readJSON(name string, out any) error {
	data, err := os.ReadFile(filepath.Join(b.Dir, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (b Book) writeJSON(name string, value any, perm os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(b.Dir, name), data, perm)
}

func (b Book) writeContacts(contacts map[string]Contact) error {
	if err := os.MkdirAll(b.Dir, 0o700); err != nil {
		return err
	}
	return b.writeJSON("contacts.json", contacts, 0o600)
}
