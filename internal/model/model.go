package model

// Note is one labeled note on an identity.
type Note struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Identity is a profile you speak as. The Ed25519 key signs markdown-direct messages.
// The private key lives beside the profile in the store.
type Identity struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Username string   `json:"username,omitempty"`
	Pronouns string   `json:"pronouns,omitempty"`
	Notes    []Note   `json:"notes,omitempty"`
	Services []string `json:"services,omitempty"`
	Icon     string   `json:"icon,omitempty"`
	Public   string   `json:"public"`
}

// Service is one configured protocol endpoint owned by one identity.
type Service struct {
	ID         string            `json:"id"`
	Plugin     string            `json:"plugin"`
	IdentityID string            `json:"identityID"`
	Config     map[string]string `json:"config"`
	Status     string            `json:"status"`
}

// Field is one value a plugin needs before it can open a connection.
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
	Bool        bool   `json:"bool,omitempty"`
}

// Context is a service connection an account is on, or a pane view.
// Plugin and Config are the connection. A pane view leaves Plugin empty.
type Context struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	ServiceID  string            `json:"serviceID,omitempty"`
	IdentityID string            `json:"identityID,omitempty"`
	Plugin     string            `json:"plugin,omitempty"`
	Config     map[string]string `json:"config,omitempty"`
	TargetKind string            `json:"targetKind,omitempty"`
	Target     string            `json:"target,omitempty"`
	LastReadID string            `json:"lastReadID,omitempty"`
	Draft      string            `json:"draft,omitempty"`
}

const (
	KindConversation = "conversation"
	KindSelect       = "select"
	KindInspector    = "inspector"
	KindPanes        = "panes"
	TargetUser       = "user"
	TargetGroup      = "group"
	PluginLocal      = "local"
	PluginDirect     = "markdown-direct"
	PluginIRC        = "irc"
	PluginXMPP       = "xmpp"
)

// Node is a pane or a split. Ratios are fractions of the parent.
type Node struct {
	Kind          string  `json:"kind"`
	Dir           string  `json:"dir,omitempty"`
	Ratio         float64 `json:"ratio,omitempty"`
	Leading       *Node   `json:"leading,omitempty"`
	Trailing      *Node   `json:"trailing,omitempty"`
	ID            string  `json:"id,omitempty"`
	ContextID     string  `json:"contextID,omitempty"`
	HeaderVisible *bool   `json:"headerVisible,omitempty"`
	SkinID        string  `json:"skinID,omitempty"`
}

func (n *Node) ShowHeader() bool {
	if n == nil || n.HeaderVisible == nil {
		return true
	}
	return *n.HeaderVisible
}

func BoolPtr(v bool) *bool { return &v }

// PluginRef is an external plugin binary.
type PluginRef struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	Enabled      bool     `json:"enabled"`
	Marks        []string `json:"marks,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
	Fields       []Field  `json:"fields,omitempty"`
}

// Theme is the persisted color, type, and mark-order settings.
type Theme struct {
	Preset    string            `json:"preset"`
	Colors    map[string]string `json:"colors"`
	FontScale float64           `json:"fontScale"`
	Fonts     map[string]string `json:"fonts"`
	MarkOrder []string          `json:"markOrder"`
}

// SelectState remembers which selection lists are open and how they are filtered.
type SelectState struct {
	UsersOpen  bool `json:"usersOpen"`
	GroupsOpen bool `json:"groupsOpen"`
	ChatsOpen  bool `json:"chatsOpen"`
	// Scope is mine, all, or group. Empty means all.
	Scope string `json:"scope,omitempty"`
	// GroupOpen is whether an account section is expanded in group scope. Missing means open.
	GroupOpen map[string]bool `json:"groupOpen,omitempty"`
}

// Config is the root persisted document.
type Config struct {
	Layout        *Node   `json:"layout"`
	SettingsRatio float64 `json:"settingsRatio"`
	SplitHandlePx float64 `json:"splitHandlePx"`
	Skin          string  `json:"skin"`
	// IgnoreSkinTheme keeps the Settings → Colors theme even when the skin has a theme block.
	IgnoreSkinTheme bool `json:"ignoreSkinTheme,omitempty"`
	// PanesWindow is true while the Panes list is popped out into its own window.
	PanesWindow   bool              `json:"panesWindow,omitempty"`
	Theme         Theme             `json:"theme"`
	Keys          map[string]string `json:"keys"`
	Select        SelectState       `json:"select"`
	Plugins       []PluginRef       `json:"plugins"`
	Passphrase    bool              `json:"passphrase"`
	FocusedPane   string            `json:"focusedPane,omitempty"`
	Handshake     map[string]string `json:"handshake,omitempty"`
	ComposerLines float64           `json:"composerLines,omitempty"`
	ComposerBar   float64           `json:"composerBar,omitempty"`
	// Enabled turns a keybinding on or off. A missing name stays on.
	Enabled map[string]bool `json:"enabled,omitempty"`
}

// Pin is a trusted markdown-direct key for an address.
type Pin struct {
	Address string `json:"address"`
	Key     string `json:"key"`
	Name    string `json:"name,omitempty"`
}

// LogMessage is one user message in a conversation log.
type LogMessage struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	Time    string `json:"time"`
	Sig     string `json:"sig,omitempty"`
	Service string `json:"service,omitempty"`
	Body    string `json:"body"`
}

// Control is a lonchan fenced block that is not transcript text.
type Control struct {
	Type   string   `json:"type"`
	Marks  []string `json:"marks,omitempty"`
	Name   string   `json:"name,omitempty"`
	ID     string   `json:"id,omitempty"`
	State  string   `json:"state,omitempty"`
	Room   string   `json:"room,omitempty"`
	Origin string   `json:"origin,omitempty"`
}

// Label returns the display name, then the username, then the public-key prefix.
func (id Identity) Label() string {
	if id.Name != "" {
		return id.Name
	}
	if id.Username != "" {
		return id.Username
	}
	if len(id.Public) >= 8 {
		return id.Public[:8]
	}
	return id.Public
}

// Prefix is the short public-key prefix used to confirm deletion.
func (id Identity) Prefix() string {
	if len(id.Public) >= 8 {
		return id.Public[:8]
	}
	return id.Public
}
