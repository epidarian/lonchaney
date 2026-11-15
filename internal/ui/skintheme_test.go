package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/hub"
	"lonchan/internal/model"
	"lonchan/internal/skin"
	"lonchan/internal/store"
)

func TestSkinTheme(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pack := skin.Pack{
		Name:    "t",
		Palette: map[string]string{"ink": "#FFCC99"},
		Theme: &skin.Theme{
			Variant: "dark",
			Colors:  map[string]string{"background": "#000000", "foreground": "ink", "button": "#3D2E5C"},
			Sizes:   map[string]float64{"inputRadius": 16},
		},
		Panes: map[string]skin.PaneSkin{
			"inspector": {Theme: &skin.Theme{Colors: map[string]string{"foreground": "#CCCCFF"}}},
		},
	}
	b, _ := json.Marshal(pack)
	if err := os.MkdirAll(filepath.Join(dir, "skins", "t"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skins", "t", "skin.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = st.UpdateConfig(func(c *model.Config) { c.Skin = "t" })

	a := test.NewApp()
	w := a.NewWindow("t")
	u := &UI{app: a, win: w, hub: hub.New(st), scrollOff: map[string]fyne.Position{}, focused: "pane-chat"}
	u.applyTheme()
	th := a.Settings().Theme()
	if got := th.Color(theme.ColorNameForeground, theme.VariantLight); got != skin.ParseHex("#FFCC99") {
		t.Fatalf("skin foreground %v", got)
	}
	if got := th.Color(theme.ColorNameButton, theme.VariantLight); got != skin.ParseHex("#3D2E5C") {
		t.Fatalf("skin button %v", got)
	}
	if got := th.Size(theme.SizeNameInputRadius); got != 16 {
		t.Fatalf("skin radius %v", got)
	}
	// Names the skin leaves out come from Fyne's dark palette, not the user's Light preset.
	if got, want := th.Color(theme.ColorNameInputBackground, theme.VariantLight), theme.DefaultTheme().Color(theme.ColorNameInputBackground, theme.VariantDark); got != want {
		t.Fatalf("unset name %v, want dark %v", got, want)
	}

	// A slot theme reaches the controls placed in that slot.
	label := widget.NewLabel("x")
	loaded, _ := skin.Load(filepath.Join(dir, "skins", "t"))
	frame := u.newFrame(loaded.Pack, loaded.Dir, "inspector", nil, label)
	w.SetContent(frame)
	if got := theme.ColorForWidget(theme.ColorNameForeground, label); got != skin.ParseHex("#CCCCFF") {
		t.Fatalf("slot foreground %v", got)
	}

	_ = st.UpdateConfig(func(c *model.Config) { c.IgnoreSkinTheme = true })
	u.applyTheme()
	if got := a.Settings().Theme().Color(theme.ColorNameForeground, theme.VariantLight); got != skin.ParseHex("#1A1A1A") {
		t.Fatalf("ignored skin theme still applied: %v", got)
	}
}
