package ui

import (
	"testing"

	"lonchan/internal/model"
)

func TestScopeOf(t *testing.T) {
	if scopeOf("") != scopeAll || scopeOf("nope") != scopeAll {
		t.Fatal("empty and unknown scopes are all")
	}
	if scopeOf(scopeMine) != scopeMine || scopeOf(scopeGroup) != scopeGroup {
		t.Fatal("mine and group stay")
	}
	choices := scopeChoices()
	if len(choices) != 3 {
		t.Fatalf("choices %d", len(choices))
	}
	seen := map[string]bool{}
	for _, c := range choices {
		if c.label == "" || c.tip == "" || seen[c.id] {
			t.Fatalf("choice %+v", c)
		}
		seen[c.id] = true
	}
}

func TestRowListed(t *testing.T) {
	jason := selectRow{title: "Will", kind: "user", ctx: model.Context{IdentityID: "jason"}}
	will := selectRow{title: "Jason", kind: "user", ctx: model.Context{IdentityID: "will"}}
	pending := selectRow{title: "abcd", kind: "user", pending: true}
	chat := selectRow{title: "Will", kind: "chat", ctx: model.Context{IdentityID: "jason"}}

	if !rowListed(jason, "user", "", scopeMine, "jason", "", false) {
		t.Fatal("mine keeps the focused account")
	}
	if rowListed(will, "user", "", scopeMine, "jason", "", false) {
		t.Fatal("mine hides the other account")
	}
	if rowListed(pending, "user", "", scopeMine, "jason", "", false) {
		t.Fatal("mine hides a pending peer")
	}
	if !rowListed(will, "user", "", scopeMine, "", "", false) || !rowListed(pending, "user", "", scopeMine, "", "", false) {
		t.Fatal("mine with no focused account shows every row")
	}
	if !rowListed(jason, "user", "", scopeAll, "jason", "", false) || !rowListed(will, "user", "", scopeAll, "jason", "", false) {
		t.Fatal("all keeps both accounts")
	}
	if rowListed(chat, "user", "", scopeAll, "", "", false) {
		t.Fatal("a chat row is not a user row")
	}
	if !rowListed(jason, "user", "jason", scopeAll, "", "", false) || rowListed(will, "user", "jason", scopeAll, "", "", false) {
		t.Fatal("an account section keeps only that account")
	}
	if rowListed(pending, "user", "jason", scopeAll, "", "", false) || !rowListed(pending, "user", "", scopeAll, "", "", true) {
		t.Fatal("pending rows stay out of an account and in the pending section")
	}
	if rowListed(jason, "user", "", scopeAll, "", "nope", false) || !rowListed(jason, "user", "", scopeAll, "", "will", false) {
		t.Fatal("the filter matches the title")
	}
}
