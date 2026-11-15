package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/nacl/secretbox"

	"lonchan/internal/icon"
	"lonchan/internal/layout"
	"lonchan/internal/model"
)

// Store is the JSON and markdown tree under the config directory.
type Store struct {
	mu   sync.Mutex
	dir  string
	cfg  model.Config
	ids  []model.Identity
	svcs []model.Service
	ctxs []model.Context
	pins []model.Pin
	pass string
	keys map[string]ed25519.PrivateKey
}

func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	next := filepath.Join(base, "lonchan")
	if st, err := os.Stat(next); err == nil && st.IsDir() {
		return next, nil
	}
	prev := filepath.Join(base, "lonchaney")
	if st, err := os.Stat(prev); err == nil && st.IsDir() {
		return prev, nil
	}
	return next, nil
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, keys: map[string]ed25519.PrivateKey{}}
	cfgPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := s.seed(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Config() model.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *Store) Identities() []model.Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]model.Identity(nil), s.ids...)
	return out
}

func (s *Store) Services() []model.Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Service(nil), s.svcs...)
}

func (s *Store) Contexts() []model.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Context(nil), s.ctxs...)
}

func (s *Store) Pins() []model.Pin {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Pin(nil), s.pins...)
}

func (s *Store) NeedsUnlock() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Passphrase && len(s.keys) == 0
}

func (s *Store) seed() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	id := model.Identity{ID: newID(), Public: hex.EncodeToString(pub), Services: []string{"svc-local", "svc-direct"}}
	local := model.Service{ID: "svc-local", Plugin: model.PluginLocal, IdentityID: id.ID, Config: map[string]string{}}
	direct := model.Service{ID: "svc-direct", Plugin: model.PluginDirect, IdentityID: id.ID, Config: map[string]string{"bind": "", "relay": ""}}
	sel := model.Context{ID: "ctx-select", Name: "Selection", Kind: model.KindSelect}
	ins := model.Context{ID: "ctx-inspector", Name: "Inspector", Kind: model.KindInspector}
	panes := model.Context{ID: PanesContextID, Name: "Panes", Kind: model.KindPanes}
	chat := model.Context{ID: "ctx-chat", Name: "Chat", Kind: model.KindConversation, ServiceID: local.ID, IdentityID: id.ID}
	localConn := connectionContext(local)
	directConn := connectionContext(direct)
	s.ids = []model.Identity{id}
	s.svcs = []model.Service{local, direct}
	s.ctxs = []model.Context{sel, panes, ins, chat, localConn, directConn}
	s.cfg = defaultConfig(layout.Default(sel.ID, chat.ID, panes.ID))
	s.keys[id.ID] = priv
	return s.flush()
}

func defaultConfig(tree *model.Node) model.Config {
	return model.Config{
		Layout:        tree,
		SettingsRatio: 0.20,
		SplitHandlePx: 4,
		Skin:          "plain",
		Theme:         DefaultTheme("Light"),
		Keys:          DefaultKeys(),
		Select:        model.SelectState{UsersOpen: true, GroupsOpen: true, ChatsOpen: true},
		Handshake:     map[string]string{},
	}
}

func DefaultKeys() map[string]string {
	return map[string]string{
		"settings":   "Ctrl+,",
		"bold":       "Ctrl+B",
		"italic":     "Ctrl+I",
		"strike":     "Ctrl+Shift+X",
		"code":       "Ctrl+E",
		"code_block": "Ctrl+Shift+E",
		"link":       "Ctrl+K",
		"heading":    "Ctrl+H",
		"quote":      "Ctrl+Shift+Q",
		"list":       "Ctrl+L",
		"send":       "Return",
		"newline":    "Shift+Return",
		"next":       "Ctrl+Tab",
		"previous":   "Ctrl+Shift+Tab",
	}
}

func DefaultTheme(preset string) model.Theme {
	colors := lightColors()
	if preset == "Dark" {
		colors = darkColors()
	}
	return model.Theme{
		Preset:    preset,
		Colors:    colors,
		FontScale: 1,
		Fonts:     map[string]string{"regular": "", "bold": "", "italic": "", "monospace": ""},
		MarkOrder: []string{"bold", "italic", "strike", "code", "code_block", "link", "heading", "quote", "list"},
	}
}

func lightColors() map[string]string {
	return map[string]string{
		"background": "#FFFFFF", "foreground": "#1A1A1A", "primary": "#1B5EBF",
		"disabled": "#9A9A9A", "selection": "#C5D8F5", "inputBackground": "#F4F4F4",
		"separator": "#D0D0D0", "hover": "#E6E6E6", "error": "#9E2B2B", "success": "#1E7A46",
	}
}

func darkColors() map[string]string {
	return map[string]string{
		"background": "#1C1C1C", "foreground": "#E8E8E8", "primary": "#7AABF5",
		"disabled": "#6E6E6E", "selection": "#2A4066", "inputBackground": "#2A2A2A",
		"separator": "#3A3A3A", "hover": "#333333", "error": "#E07070", "success": "#7DCEA0",
	}
}

func (s *Store) load() error {
	b, err := os.ReadFile(filepath.Join(s.dir, "config.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &s.cfg); err != nil {
		return err
	}
	if s.cfg.Handshake == nil {
		s.cfg.Handshake = map[string]string{}
	}
	s.ids, err = readSlice[model.Identity](filepath.Join(s.dir, "identities.json"))
	if err != nil {
		return err
	}
	s.svcs, err = readSlice[model.Service](filepath.Join(s.dir, "services.json"))
	if err != nil {
		return err
	}
	s.ctxs, err = readSlice[model.Context](filepath.Join(s.dir, "contexts.json"))
	if err != nil {
		return err
	}
	s.pins, err = readSlice[model.Pin](filepath.Join(s.dir, "pins.json"))
	if err != nil {
		return err
	}
	if !s.cfg.Passphrase {
		for _, id := range s.ids {
			key, err := s.readKey(id.ID, "")
			if err != nil {
				return err
			}
			s.keys[id.ID] = key
		}
	}
	s.repairServiceMap()
	if err := s.repairConnections(); err != nil {
		return err
	}
	s.ensurePanesContext()
	return nil
}

// connectionContext is the context that contains one service connection.
func connectionContext(svc model.Service) model.Context {
	cfg := map[string]string{}
	for k, v := range svc.Config {
		cfg[k] = v
	}
	return model.Context{
		ID: "ctx-conn-" + svc.ID, Name: svc.Plugin, Kind: model.KindConversation,
		ServiceID: svc.ID, IdentityID: svc.IdentityID, Plugin: svc.Plugin, Config: cfg,
	}
}

// repairConnections gives every service a context that contains it.
func (s *Store) repairConnections() error {
	var add []model.Context
	for _, svc := range s.svcs {
		owned := false
		for _, c := range s.ctxs {
			if c.Plugin != "" && c.ServiceID == svc.ID {
				owned = true
				break
			}
		}
		if !owned {
			add = append(add, connectionContext(svc))
		}
	}
	if len(add) == 0 {
		return nil
	}
	s.ctxs = append(s.ctxs, add...)
	return writeJSON(filepath.Join(s.dir, "contexts.json"), s.ctxs)
}

// BindingOn reports whether a keybinding is enabled. A missing entry stays on.
func BindingOn(cfg model.Config, name string) bool {
	if cfg.Enabled == nil {
		return true
	}
	on, set := cfg.Enabled[name]
	if !set {
		return true
	}
	return on
}

func (s *Store) flush() error {
	if err := writeJSON(filepath.Join(s.dir, "config.json"), s.cfg); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, "identities.json"), s.ids); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, "services.json"), s.svcs); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, "contexts.json"), s.ctxs); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, "pins.json"), s.pins); err != nil {
		return err
	}
	for id, key := range s.keys {
		if err := s.writeKey(id, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

func (s *Store) UpdateConfig(fn func(*model.Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	return writeJSON(filepath.Join(s.dir, "config.json"), s.cfg)
}

func (s *Store) PutContext(c model.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ctxs {
		if s.ctxs[i].ID == c.ID {
			s.ctxs[i] = c
			return writeJSON(filepath.Join(s.dir, "contexts.json"), s.ctxs)
		}
	}
	s.ctxs = append(s.ctxs, c)
	return writeJSON(filepath.Join(s.dir, "contexts.json"), s.ctxs)
}

func (s *Store) DeleteService(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.svcs[:0]
	for _, svc := range s.svcs {
		if svc.ID != id {
			next = append(next, svc)
		}
	}
	s.svcs = next
	s.noteServiceOwner(id, "")
	if err := writeJSON(filepath.Join(s.dir, "services.json"), s.svcs); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
}

func (s *Store) DeleteContext(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.ctxs[:0]
	for _, c := range s.ctxs {
		if c.ID != id {
			next = append(next, c)
		}
	}
	s.ctxs = next
	return writeJSON(filepath.Join(s.dir, "contexts.json"), s.ctxs)
}

func (s *Store) PutService(svc model.Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if svc.Config == nil {
		svc.Config = map[string]string{}
	}
	found := false
	for i := range s.svcs {
		if s.svcs[i].ID == svc.ID {
			s.svcs[i] = svc
			found = true
			break
		}
	}
	if !found {
		s.svcs = append(s.svcs, svc)
	}
	s.noteServiceOwner(svc.ID, svc.IdentityID)
	if err := writeJSON(filepath.Join(s.dir, "services.json"), s.svcs); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
}

// AssignService adds or removes a service from an identity's mapping.
func (s *Store) AssignService(identityID, serviceID string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	owner := ""
	for i := range s.svcs {
		if s.svcs[i].ID != serviceID {
			continue
		}
		found = true
		if on {
			s.svcs[i].IdentityID = identityID
			owner = identityID
		} else if s.svcs[i].IdentityID == identityID {
			s.svcs[i].IdentityID = ""
		} else {
			owner = s.svcs[i].IdentityID
		}
	}
	if !found {
		return errors.New("service not found")
	}
	s.noteServiceOwner(serviceID, owner)
	if err := writeJSON(filepath.Join(s.dir, "services.json"), s.svcs); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
}

// noteServiceOwner records which identity lists this service. The caller holds s.mu.
func (s *Store) noteServiceOwner(serviceID, identityID string) {
	for i := range s.ids {
		var keep []string
		for _, sid := range s.ids[i].Services {
			if sid != serviceID {
				keep = append(keep, sid)
			}
		}
		if s.ids[i].ID == identityID && serviceID != "" {
			keep = append(keep, serviceID)
		}
		s.ids[i].Services = keep
	}
}

func (s *Store) repairServiceMap() {
	for _, svc := range s.svcs {
		if svc.IdentityID == "" {
			continue
		}
		s.noteServiceOwner(svc.ID, svc.IdentityID)
	}
}

func (s *Store) SetServiceStatus(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.svcs {
		if s.svcs[i].ID == id {
			s.svcs[i].Status = status
		}
	}
}

func (s *Store) AddIdentity(name string) (model.Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return model.Identity{}, err
	}
	id := model.Identity{ID: newID(), Name: name, Public: hex.EncodeToString(pub)}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids = append(s.ids, id)
	s.keys[id.ID] = priv
	if err := writeJSON(filepath.Join(s.dir, "identities.json"), s.ids); err != nil {
		return model.Identity{}, err
	}
	if err := s.writeKey(id.ID, priv); err != nil {
		return model.Identity{}, err
	}
	return id, nil
}

func (s *Store) RenameIdentity(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ids {
		if s.ids[i].ID == id {
			s.ids[i].Name = name
			return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
		}
	}
	return errors.New("identity not found")
}

// PutIdentity writes the profile fields. The public key stays the one already stored.
func (s *Store) PutIdentity(id model.Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ids {
		if s.ids[i].ID == id.ID {
			id.Public = s.ids[i].Public
			s.ids[i] = id
			return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
		}
	}
	return errors.New("identity not found")
}

// SetIcon stores an icon file for the identity. filename supplies the format.
func (s *Store) SetIcon(id, filename string, data []byte) error {
	_, ext, err := icon.Decode(filename, data)
	if err != nil {
		return err
	}
	rel := filepath.ToSlash(filepath.Join("icons", id+ext))
	abs := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ids {
		if s.ids[i].ID != id {
			continue
		}
		if old := s.ids[i].Icon; old != "" && old != rel {
			_ = os.Remove(filepath.Join(s.dir, filepath.FromSlash(old)))
		}
		s.ids[i].Icon = rel
		return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
	}
	return errors.New("identity not found")
}

// ClearIcon removes the identity's icon file.
func (s *Store) ClearIcon(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ids {
		if s.ids[i].ID != id {
			continue
		}
		if s.ids[i].Icon != "" {
			_ = os.Remove(filepath.Join(s.dir, filepath.FromSlash(s.ids[i].Icon)))
		}
		s.ids[i].Icon = ""
		return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
	}
	return errors.New("identity not found")
}

// AbsIcon returns the icon file path, or "" when the identity has none.
func (s *Store) AbsIcon(rel string) string {
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(s.dir, filepath.FromSlash(rel))
}

func (s *Store) DeleteIdentity(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var iconRel string
	next := s.ids[:0]
	for _, item := range s.ids {
		if item.ID == id {
			iconRel = item.Icon
			continue
		}
		next = append(next, item)
	}
	s.ids = next
	for i := range s.svcs {
		if s.svcs[i].IdentityID == id {
			s.svcs[i].IdentityID = ""
		}
	}
	delete(s.keys, id)
	_ = os.Remove(filepath.Join(s.dir, "keys", id+".json"))
	if iconRel != "" {
		_ = os.Remove(filepath.Join(s.dir, filepath.FromSlash(iconRel)))
	}
	if err := writeJSON(filepath.Join(s.dir, "services.json"), s.svcs); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, "identities.json"), s.ids)
}

func (s *Store) Private(id string) (ed25519.PrivateKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.keys[id]
	if !ok {
		return nil, errors.New("identity key is locked")
	}
	return key, nil
}

func (s *Store) Identity(id string) (model.Identity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.ids {
		if item.ID == id {
			return item, true
		}
	}
	return model.Identity{}, false
}

func (s *Store) Service(id string) (model.Service, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.svcs {
		if item.ID == id {
			return item, true
		}
	}
	return model.Service{}, false
}

func (s *Store) Context(id string) (model.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.ctxs {
		if item.ID == id {
			return item, true
		}
	}
	return model.Context{}, false
}

func (s *Store) PutPin(p model.Pin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.pins {
		if s.pins[i].Address == p.Address {
			s.pins[i] = p
			return writeJSON(filepath.Join(s.dir, "pins.json"), s.pins)
		}
	}
	s.pins = append(s.pins, p)
	return writeJSON(filepath.Join(s.dir, "pins.json"), s.pins)
}

func (s *Store) DeletePin(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.pins[:0]
	for _, p := range s.pins {
		if p.Address != address {
			next = append(next, p)
		}
	}
	s.pins = next
	return writeJSON(filepath.Join(s.dir, "pins.json"), s.pins)
}

func (s *Store) Pin(address string) (model.Pin, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pins {
		if p.Address == address {
			return p, true
		}
	}
	return model.Pin{}, false
}

func (s *Store) SetHandshake(identityID, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.Handshake == nil {
		s.cfg.Handshake = map[string]string{}
	}
	s.cfg.Handshake[identityID] = text
	_ = writeJSON(filepath.Join(s.dir, "config.json"), s.cfg)
}

func (s *Store) LogPath(serviceID, target string) string {
	key := serviceID + "_" + target
	if target == "" {
		key = serviceID + "_draft"
	}
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '_'
		}
		return r
	}, key)
	return filepath.Join(s.dir, "conversations", safe, "log.md")
}

func (s *Store) AppendLog(serviceID, target string, msg model.LogMessage) error {
	return AppendLog(s.LogPath(serviceID, target), msg)
}

func (s *Store) ReadLog(serviceID, target string) ([]model.LogMessage, error) {
	return ReadLog(s.LogPath(serviceID, target))
}

func (s *Store) Wrap(pass string) error {
	if pass == "" {
		return errors.New("empty passphrase")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pass = pass
	s.cfg.Passphrase = true
	for id, key := range s.keys {
		if err := s.writeKey(id, key); err != nil {
			return err
		}
	}
	return writeJSON(filepath.Join(s.dir, "config.json"), s.cfg)
}

func (s *Store) Unlock(pass string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := map[string]ed25519.PrivateKey{}
	for _, id := range s.ids {
		key, err := s.readKey(id.ID, pass)
		if err != nil {
			return err
		}
		keys[id.ID] = key
	}
	s.keys = keys
	s.pass = pass
	return nil
}

func (s *Store) ClearWrap(pass string) error {
	if err := s.Unlock(pass); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Passphrase = false
	s.pass = ""
	for id, key := range s.keys {
		if err := s.writeKey(id, key); err != nil {
			return err
		}
	}
	return writeJSON(filepath.Join(s.dir, "config.json"), s.cfg)
}

type keyFile struct {
	Wrapped bool   `json:"wrapped"`
	Seed    string `json:"seed,omitempty"`
	Salt    string `json:"salt,omitempty"`
	Nonce   string `json:"nonce,omitempty"`
	Box     string `json:"box,omitempty"`
}

func (s *Store) writeKey(id string, key ed25519.PrivateKey) error {
	dir := filepath.Join(s.dir, "keys")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	kf := keyFile{}
	if s.cfg.Passphrase {
		salt := make([]byte, 16)
		nonce := make([]byte, 24)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
		if _, err := rand.Read(nonce); err != nil {
			return err
		}
		secret := argon2.IDKey([]byte(s.pass), salt, 1, 16*1024, 1, 32)
		var n [24]byte
		var sec [32]byte
		copy(n[:], nonce)
		copy(sec[:], secret)
		box := secretbox.Seal(nil, key, &n, &sec)
		kf.Wrapped = true
		kf.Salt = hex.EncodeToString(salt)
		kf.Nonce = hex.EncodeToString(nonce)
		kf.Box = hex.EncodeToString(box)
	} else {
		kf.Seed = hex.EncodeToString(key)
	}
	return writeJSON(filepath.Join(dir, id+".json"), kf)
}

func (s *Store) readKey(id, pass string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(filepath.Join(s.dir, "keys", id+".json"))
	if err != nil {
		return nil, err
	}
	var kf keyFile
	if err := json.Unmarshal(b, &kf); err != nil {
		return nil, err
	}
	if !kf.Wrapped {
		raw, err := hex.DecodeString(kf.Seed)
		if err != nil {
			return nil, err
		}
		return ed25519.PrivateKey(raw), nil
	}
	salt, err := hex.DecodeString(kf.Salt)
	if err != nil {
		return nil, err
	}
	nonce, err := hex.DecodeString(kf.Nonce)
	if err != nil {
		return nil, err
	}
	box, err := hex.DecodeString(kf.Box)
	if err != nil {
		return nil, err
	}
	secret := argon2.IDKey([]byte(pass), salt, 1, 16*1024, 1, 32)
	var n [24]byte
	var sec [32]byte
	copy(n[:], nonce)
	copy(sec[:], secret)
	plain, ok := secretbox.Open(nil, box, &n, &sec)
	if !ok {
		return nil, errors.New("passphrase did not open the key")
	}
	return ed25519.PrivateKey(plain), nil
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func readSlice[T any](path string) ([]T, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []T
	if len(b) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PanesContextID is the built-in context that shows the Panes list.
const PanesContextID = "ctx-panes"

// ensurePanesContext adds the Panes context to stores made before it existed.
func (s *Store) ensurePanesContext() {
	for _, c := range s.ctxs {
		if c.ID == PanesContextID {
			return
		}
	}
	s.ctxs = append(s.ctxs, model.Context{ID: PanesContextID, Name: "Panes", Kind: model.KindPanes})
}
