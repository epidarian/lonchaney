package layout

import "lonchan/internal/model"

const (
	OuterRatio = 0.10
	InnerRatio = 0.8 / 0.9
)

// Default is the 10 | 80 | 10 tree: selection, conversation, and the Panes list.
func Default(selectID, chatID, rightID string) *model.Node {
	show := model.BoolPtr(true)
	return &model.Node{
		Kind:  "split",
		Dir:   "h",
		Ratio: OuterRatio,
		Leading: &model.Node{
			Kind: "pane", ID: "pane-select", ContextID: selectID, HeaderVisible: show,
		},
		Trailing: &model.Node{
			Kind:  "split",
			Dir:   "h",
			Ratio: InnerRatio,
			Leading: &model.Node{
				Kind: "pane", ID: "pane-chat", ContextID: chatID, HeaderVisible: show,
			},
			Trailing: &model.Node{
				Kind: "pane", ID: "pane-panes", ContextID: rightID, HeaderVisible: show,
			},
		},
	}
}

// Panes returns pane nodes in tree order.
func Panes(n *model.Node) []*model.Node {
	if n == nil {
		return nil
	}
	if n.Kind == "pane" {
		return []*model.Node{n}
	}
	out := Panes(n.Leading)
	out = append(out, Panes(n.Trailing)...)
	return out
}

// FindPane returns the pane with the given id.
func FindPane(n *model.Node, id string) *model.Node {
	for _, p := range Panes(n) {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// WidestConversation returns the pane id whose context kind is conversation
// and whose share of the root width is largest. share is computed for horizontal splits only.
func WidestConversation(n *model.Node, kindOf func(contextID string) string) string {
	type item struct {
		id    string
		share float64
	}
	var best item
	var walk func(node *model.Node, share float64)
	walk = func(node *model.Node, share float64) {
		if node == nil {
			return
		}
		if node.Kind == "pane" {
			if kindOf(node.ContextID) == model.KindConversation && share > best.share {
				best = item{id: node.ID, share: share}
			}
			return
		}
		lead, trail := share, share
		if node.Dir == "h" || node.Dir == "" {
			lead = share * node.Ratio
			trail = share * (1 - node.Ratio)
		}
		walk(node.Leading, lead)
		walk(node.Trailing, trail)
	}
	walk(n, 1)
	return best.id
}

// RemovePane promotes the sibling of paneID and drops that pane.
// The root pane is left in place.
func RemovePane(root *model.Node, paneID string) *model.Node {
	if root == nil || root.Kind == "pane" {
		return root
	}
	return remove(root, paneID)
}

func remove(n *model.Node, paneID string) *model.Node {
	if n == nil || n.Kind == "pane" {
		return n
	}
	if n.Leading != nil && n.Leading.Kind == "pane" && n.Leading.ID == paneID {
		return n.Trailing
	}
	if n.Trailing != nil && n.Trailing.Kind == "pane" && n.Trailing.ID == paneID {
		return n.Leading
	}
	n.Leading = remove(n.Leading, paneID)
	n.Trailing = remove(n.Trailing, paneID)
	return n
}

// DivideLeading replaces the leading child with a split of a new pane and that child.
func DivideLeading(n *model.Node, pane *model.Node) {
	if n == nil || n.Kind != "split" {
		return
	}
	n.Leading = &model.Node{
		Kind: "split", Dir: n.Dir, Ratio: 0.5,
		Leading:  pane,
		Trailing: n.Leading,
	}
}

// DivideTrailing replaces the trailing child with a split of a new pane and that child.
func DivideTrailing(n *model.Node, pane *model.Node) {
	if n == nil || n.Kind != "split" {
		return
	}
	n.Trailing = &model.Node{
		Kind: "split", Dir: n.Dir, Ratio: 0.5,
		Leading:  pane,
		Trailing: n.Trailing,
	}
}

// RemoveSplit promotes the leading child of the split that matches the pointer.
func RemoveSplit(root, split *model.Node) *model.Node {
	if root == nil || split == nil || root == split {
		if split != nil && split.Leading != nil {
			return split.Leading
		}
		return root
	}
	if root.Kind != "split" {
		return root
	}
	if root.Leading == split {
		root.Leading = split.Leading
		return root
	}
	if root.Trailing == split {
		root.Trailing = split.Leading
		return root
	}
	root.Leading = RemoveSplit(root.Leading, split)
	root.Trailing = RemoveSplit(root.Trailing, split)
	return root
}

// SplitPane turns the pane paneID into a split of that pane and newPane.
// dir is "h" for side by side (newPane on the right) or "v" for stacked
// (newPane below). It reports whether the pane was found.
func SplitPane(root *model.Node, paneID string, newPane *model.Node, dir string) bool {
	p := FindPane(root, paneID)
	if p == nil || newPane == nil {
		return false
	}
	old := *p
	*p = model.Node{Kind: "split", Dir: dir, Ratio: 0.5, Leading: &old, Trailing: newPane}
	return true
}
