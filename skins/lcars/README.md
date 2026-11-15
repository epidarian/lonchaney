# LCARS skin

An LCARS-style skin: a black field framed by segmented sidebars, rounded elbows, and pill-capped bars.

- **Conversation:** a wide sidebar with two elbows. The header sits above the top bar, the transcript sits between the bars, and the composer sits above the bottom bar.
- **Selection and Inspector:** a thin rail down the outer edge that turns into a bar along the bottom.
- **Settings:** a rail down the sidebar's right edge, and an elbow in the top-right corner of each page.
- **Split handles:** black gaps between the panes.

Everything is drawn with `rect` operations. Each elbow is three shapes: a rounded block for the outer curve, square bars, and a black rounded block that cuts the inner curve.

## Controls

The skin restyles the controls too, so it works with any Settings → Colors preset:

- Tan text on black.
- Orange primary buttons with black text, and violet standard buttons.
- Pill-shaped buttons and fields.
- Dark navy fields, menus, and pop-ups.
- The Selection pane leans orange for selected rows and unread badges. The Inspector uses lavender text.

To keep your own Settings → Colors theme with this skin's drawing, clear **Use the skin's colors, sizes, fonts, and icons** under Settings → Skins.

## Install

Copy this folder to `%AppData%\lonchan\skins\lcars`, or use Settings → Skins → **Add folder…** and pick it. Then choose `lcars` under Skins.

## Editing

`skin.json` is generated. Shapes are measured in pixels on reference frames (960×780 for the conversation) in `tools/lcarsgen/main.go`, which also holds the palette and the theme blocks. Change them there and run:

```text
go run ./tools/lcarsgen skins/lcars/skin.json
```
