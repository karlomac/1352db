// Package merge implements cdb/pdb revision tree traversal and
// conflict resolution. It mirrors pdb-merge semantics: given a list of
// revisions, determine leaves, find a winning rev, and collect conflicts.
package merge

import (
	"sort"
	"strconv"
	"strings"
)

// Rev describes a single revision in a rev tree.
type Rev struct {
	Rev     string   `json:"rev"`
	Parents []string `json:"parents,omitempty"`
	Deleted bool     `json:"deleted,omitempty"`
}

// TreeInfo holds the computed rev tree information.
type TreeInfo struct {
	WinningRev string   `json:"winning_rev"`
	Conflicts  []string `json:"conflicts"`
	Leaves     []string `json:"leaves"`
}

// LeafInfo describes a leaf revision with its path to the root.
type LeafInfo struct {
	Rev     string
	Parents []string
	Deleted bool
}

// ParseRev parses a cdb revision string "N-<hash>" into its generation
// number and hash.
func ParseRev(rev string) (gen int, hash string) {
	if i := strings.IndexByte(rev, '-'); i >= 0 {
		if n, err := strconv.Atoi(rev[:i]); err == nil {
			return n, rev[i+1:]
		}
	}
	return 0, rev
}

// BuildTree builds a rev tree from a list of revisions.
func BuildTree(revs []Rev) map[string]Rev {
	tree := make(map[string]Rev, len(revs))
	for _, r := range revs {
		tree[r.Rev] = r
	}
	return tree
}

// FindLeaves returns the leaf revisions (those that are not parents of any
// other revision in the tree).
func FindLeaves(tree map[string]Rev) []LeafInfo {
	parentSet := make(map[string]bool)
	for _, r := range tree {
		for _, p := range r.Parents {
			parentSet[p] = true
		}
	}
	var leaves []LeafInfo
	for rev, r := range tree {
		if parentSet[rev] {
			continue
		}
		leaves = append(leaves, LeafInfo{
			Rev:     rev,
			Parents: append([]string{}, r.Parents...),
			Deleted: r.Deleted,
		})
	}
	sort.Slice(leaves, func(i, j int) bool {
		gi, _ := ParseRev(leaves[i].Rev)
		gj, _ := ParseRev(leaves[j].Rev)
		if gi != gj {
			return gi > gj
		}
		return leaves[i].Rev < leaves[j].Rev
	})
	return leaves
}

// FindWinningRev picks a winning leaf using cdb's rules:
// 1. Longest branch (most ancestors) wins.
// 2. Ties broken by highest generation number.
// 3. Ties broken by lexicographic rev hash.
func FindWinningRev(tree map[string]Rev) (string, []string) {
	leaves := FindLeaves(tree)
	if len(leaves) == 0 {
		return "", nil
	}
	type scored struct {
		leaf  LeafInfo
		depth int
	}
	var scoredLeaves []scored
	memo := make(map[string]int)
	for _, l := range leaves {
		depth := branchDepth(tree, l.Rev, memo, make(map[string]bool))
		scoredLeaves = append(scoredLeaves, scored{leaf: l, depth: depth})
	}
	sort.Slice(scoredLeaves, func(i, j int) bool {
		si, sj := scoredLeaves[i], scoredLeaves[j]
		if si.depth != sj.depth {
			return si.depth > sj.depth
		}
		gi, _ := ParseRev(si.leaf.Rev)
		gj, _ := ParseRev(sj.leaf.Rev)
		if gi != gj {
			return gi > gj
		}
		return si.leaf.Rev > sj.leaf.Rev
	})
	winner := scoredLeaves[0].leaf.Rev
	var conflicts []string
	for _, s := range scoredLeaves[1:] {
		conflicts = append(conflicts, s.leaf.Rev)
	}
	return winner, conflicts
}

// branchDepth returns the number of revisions on the longest path from rev to
// a root, counting rev itself.
func branchDepth(tree map[string]Rev, rev string, memo map[string]int, visiting map[string]bool) int {
	if d, ok := memo[rev]; ok {
		return d
	}
	r, ok := tree[rev]
	if !ok {
		return 1
	}
	if visiting[rev] {
		return 1
	}
	visiting[rev] = true
	maxParent := 0
	for _, p := range r.Parents {
		if _, ok := tree[p]; ok {
			if d := branchDepth(tree, p, memo, visiting); d > maxParent {
				maxParent = d
			}
		}
	}
	visiting[rev] = false
	d := maxParent + 1
	memo[rev] = d
	return d
}

// revisions as returned by _revs_info.
func TreeInfoFromRevs(revs []Rev) TreeInfo {
	tree := BuildTree(revs)
	winner, conflicts := FindWinningRev(tree)
	leaves := FindLeaves(tree)
	leafRevs := make([]string, len(leaves))
	for i, l := range leaves {
		leafRevs[i] = l.Rev
	}
	return TreeInfo{
		WinningRev: winner,
		Conflicts:  conflicts,
		Leaves:     leafRevs,
	}
}
