package ui

import (
	"image/color"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"lonchan/internal/model"
	"lonchan/internal/skin"
)

type lonTheme struct {
	base  fyne.Theme
	spec  model.Theme
	scale float32
}

func newLonTheme(spec model.Theme) fyne.Theme {
	if spec.FontScale <= 0 {
		spec.FontScale = 1
	}
	return &lonTheme{base: theme.DefaultTheme(), spec: spec, scale: float32(spec.FontScale)}
}

func (t *lonTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	key := map[fyne.ThemeColorName]string{
		theme.ColorNameBackground:      "background",
		theme.ColorNameForeground:      "foreground",
		theme.ColorNamePrimary:         "primary",
		theme.ColorNameDisabled:        "disabled",
		theme.ColorNameSelection:       "selection",
		theme.ColorNameInputBackground: "inputBackground",
		theme.ColorNameSeparator:       "separator",
		theme.ColorNameHover:           "hover",
		theme.ColorNameError:           "error",
		theme.ColorNameSuccess:         "success",
	}[name]
	if key != "" {
		if hex := t.spec.Colors[key]; hex != "" {
			return skin.ParseHex(hex)
		}
	}
	return t.base.Color(name, t.palette())
}

// palette selects the Light or Dark Fyne colors for names this theme does not store.
func (t *lonTheme) palette() fyne.ThemeVariant {
	switch t.spec.Preset {
	case "Dark":
		return theme.VariantDark
	case "Light", "":
		return theme.VariantLight
	}
	c := skin.ParseHex(t.spec.Colors["background"])
	r, g, b, _ := c.RGBA()
	lum := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535
	if lum < 0.5 {
		return theme.VariantDark
	}
	return theme.VariantLight
}

func (t *lonTheme) Font(style fyne.TextStyle) fyne.Resource {
	key := "regular"
	switch {
	case style.Monospace:
		key = "monospace"
	case style.Bold:
		key = "bold"
	case style.Italic:
		key = "italic"
	}
	if path := t.spec.Fonts[key]; path != "" {
		if res, err := fyne.LoadResourceFromPath(path); err == nil {
			return res
		}
	}
	return t.base.Font(style)
}

func (t *lonTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *lonTheme) Size(name fyne.ThemeSizeName) float32 {
	s := t.base.Size(name)
	if textSizes[name] {
		return s * t.scale
	}
	return s
}

func parseByte(s string) int {
	n, _ := strconv.Atoi(s)
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

// themeColorNames are the Fyne color names a skin may set or draw with.
var themeColorNames = map[string]bool{
	"background": true, "button": true, "disabledButton": true, "disabled": true, "error": true,
	"focus": true, "foreground": true, "foregroundOnError": true, "foregroundOnPrimary": true,
	"foregroundOnSuccess": true, "foregroundOnWarning": true, "headerBackground": true, "hover": true,
	"hyperlink": true, "inputBackground": true, "inputBorder": true, "menuBackground": true,
	"overlayBackground": true, "placeholder": true, "pressed": true, "primary": true, "scrollBar": true,
	"scrollBarBackground": true, "selection": true, "separator": true, "shadow": true, "success": true,
	"warning": true,
}

// textSizes are the size names the user's font scale multiplies.
var textSizes = map[fyne.ThemeSizeName]bool{
	theme.SizeNameText: true, theme.SizeNameHeadingText: true,
	theme.SizeNameSubHeadingText: true, theme.SizeNameCaptionText: true,
}

// skinTheme lays a skin's theme block over another theme. Names the block
// leaves out come from under.
type skinTheme struct {
	over      *skin.Theme
	pack      skin.Pack
	dir       string
	under     fyne.Theme
	variant   fyne.ThemeVariant
	pinned    bool // pass variant to under instead of the system variant
	scale     float32
	userFonts map[string]string
	files     sync.Map // path → fyne.Resource, or nil when unreadable
}

// buildTheme is the app theme: the user's Settings → Colors theme, with the
// active skin's theme block laid over it unless the user turned that off.
func (u *UI) buildTheme() fyne.Theme {
	cfg := u.hub.Store.Config()
	user := newLonTheme(cfg.Theme)
	pack, dir := u.activePack()
	if pack.Theme == nil || cfg.IgnoreSkinTheme {
		return user
	}
	t := &skinTheme{over: pack.Theme, pack: pack, dir: dir, under: user, scale: userScale(cfg.Theme), userFonts: cfg.Theme.Fonts}
	if v, ok := skinVariant(pack.Theme, pack); ok && cfg.Theme.Preset != "Custom" {
		// The skin brings its own light or dark look, so the user's Light or Dark
		// preset colors give way to Fyne's palette for that variant.
		t.under, t.variant, t.pinned = theme.DefaultTheme(), v, true
	}
	return t
}

// paneTheme is a slot's theme block over the window theme.
func paneTheme(over *skin.Theme, pack skin.Pack, dir string, cfg model.Config) fyne.Theme {
	t := &skinTheme{over: over, pack: pack, dir: dir, under: fyne.CurrentApp().Settings().Theme(), scale: userScale(cfg.Theme), userFonts: cfg.Theme.Fonts}
	if v, ok := skinVariant(over, pack); ok && over.Variant != "" {
		t.under, t.variant, t.pinned = theme.DefaultTheme(), v, true
	}
	return t
}

func userScale(spec model.Theme) float32 {
	if spec.FontScale <= 0 {
		return 1
	}
	return float32(spec.FontScale)
}

// skinVariant is the block's variant, or the one its background color implies.
func skinVariant(t *skin.Theme, pack skin.Pack) (fyne.ThemeVariant, bool) {
	switch strings.ToLower(t.Variant) {
	case "dark":
		return theme.VariantDark, true
	case "light":
		return theme.VariantLight, true
	}
	if c, ok := pack.Color(t.Colors["background"]); ok {
		lum := (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
		if lum < 0.5 {
			return theme.VariantDark, true
		}
		return theme.VariantLight, true
	}
	return 0, false
}

func (t *skinTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if s, ok := t.over.Colors[string(name)]; ok {
		if c, ok := t.pack.Color(s); ok {
			return c
		}
	}
	if t.pinned {
		v = t.variant
	}
	return t.under.Color(name, v)
}

func (t *skinTheme) Size(name fyne.ThemeSizeName) float32 {
	if s, ok := t.over.Sizes[string(name)]; ok && s >= 0 {
		if textSizes[name] {
			return float32(s) * t.scale
		}
		return float32(s)
	}
	if t.pinned && textSizes[name] {
		return t.under.Size(name) * t.scale
	}
	return t.under.Size(name)
}

func (t *skinTheme) Font(style fyne.TextStyle) fyne.Resource {
	key := "regular"
	switch {
	case style.Monospace:
		key = "monospace"
	case style.Symbol:
		key = "symbol"
	case style.Bold && style.Italic:
		key = "boldItalic"
	case style.Bold:
		key = "bold"
	case style.Italic:
		key = "italic"
	}
	userKey := key
	if key == "boldItalic" {
		userKey = "bold"
	}
	if t.userFonts[userKey] != "" {
		// A font the user picked in Settings → Colors wins over the skin's.
		return t.under.Font(style)
	}
	if res := t.file(t.over.Fonts[key], false); res != nil {
		return res
	}
	if key == "boldItalic" {
		if res := t.file(t.over.Fonts["bold"], false); res != nil {
			return res
		}
	}
	return t.under.Font(style)
}

func (t *skinTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	if res := t.file(t.over.Icons[string(name)], true); res != nil {
		return res
	}
	return t.under.Icon(name)
}

// file loads a skin-relative file once. SVG icons are themed so they follow
// the foreground color the way Fyne's own icons do.
func (t *skinTheme) file(path string, icon bool) fyne.Resource {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.dir, path)
	}
	if v, ok := t.files.Load(path); ok {
		res, _ := v.(fyne.Resource)
		return res
	}
	res, err := fyne.LoadResourceFromPath(path)
	if err != nil {
		t.files.Store(path, nil)
		return nil
	}
	if icon && strings.EqualFold(filepath.Ext(path), ".svg") {
		res = theme.NewThemedResource(res)
	}
	t.files.Store(path, res)
	return res
}
