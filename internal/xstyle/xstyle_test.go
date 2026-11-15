package xstyle

import "testing"

func TestToXEPAndMarks(t *testing.T) {
	got := ToXEP("**bold** ~~no~~")
	if got != "*bold* ~no~" {
		t.Fatalf("%q", got)
	}
	marks := Marks("*bold* _em_ ~no~ `c`")
	want := map[string]bool{"bold": true, "italic": true, "strike": true, "code": true}
	if len(marks) != len(want) {
		t.Fatalf("%v", marks)
	}
	for _, m := range marks {
		if !want[m] {
			t.Fatalf("extra %s in %v", m, marks)
		}
	}
	q := Marks("> quoted")
	if len(q) != 1 || q[0] != "quote" {
		t.Fatalf("%v", q)
	}
}
