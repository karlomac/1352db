package merge

import "testing"

func TestParseRev(t *testing.T) {
	cases := []struct {
		rev  string
		gen  int
		hash string
	}{
		{"3-abc", 3, "abc"},
		{"1-foo", 1, "foo"},
		{"12-abc-def", 12, "abc-def"},
		{"0-", 0, ""},
		{"nohash", 0, "nohash"},
		{"abc-def", 0, "abc-def"},
		{"-hash", 0, "-hash"},
	}
	for _, c := range cases {
		g, h := ParseRev(c.rev)
		if g != c.gen || h != c.hash {
			t.Errorf("ParseRev(%q) = (%d,%q), want (%d,%q)", c.rev, g, h, c.gen, c.hash)
		}
	}
}

func TestFindLeaves(t *testing.T) {
	tree := BuildTree([]Rev{
		{Rev: "1-a"},
		{Rev: "2-b", Parents: []string{"1-a"}},
		{Rev: "2-c", Parents: []string{"1-a"}},
	})
	leaves := FindLeaves(tree)
	if len(leaves) != 2 {
		t.Fatalf("leaves=%d, want 2", len(leaves))
	}
	if leaves[0].Rev != "2-b" || leaves[1].Rev != "2-c" {
		t.Errorf("leaves=%v, want [2-b 2-c]", leaves)
	}
}

func TestFindWinningRevDepth(t *testing.T) {
	tree := BuildTree([]Rev{
		{Rev: "1-a"},
		{Rev: "2-b", Parents: []string{"1-a"}},
		{Rev: "3-c", Parents: []string{"2-b"}},
		{Rev: "2-d", Parents: []string{"1-a"}},
	})
	winner, conflicts := FindWinningRev(tree)
	if winner != "3-c" {
		t.Errorf("winner=%s, want 3-c", winner)
	}
	if len(conflicts) != 1 || conflicts[0] != "2-d" {
		t.Errorf("conflicts=%v, want [2-d]", conflicts)
	}
}

func TestTreeInfoFromRevs(t *testing.T) {
	revs := []Rev{
		{Rev: "1-a"},
		{Rev: "2-b", Parents: []string{"1-a"}},
		{Rev: "3-c", Parents: []string{"2-b"}},
		{Rev: "2-d", Parents: []string{"1-a"}},
	}
	info := TreeInfoFromRevs(revs)
	if info.WinningRev != "3-c" {
		t.Errorf("WinningRev=%s, want 3-c", info.WinningRev)
	}
	if len(info.Conflicts) != 1 || info.Conflicts[0] != "2-d" {
		t.Errorf("Conflicts=%v, want [2-d]", info.Conflicts)
	}
	if len(info.Leaves) != 2 {
		t.Errorf("Leaves=%v, want 2 leaves", info.Leaves)
	}
}
