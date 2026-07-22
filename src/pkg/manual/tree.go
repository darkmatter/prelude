package manual

// treeRow is one visible sidebar line after expand/collapse filtering.
// Separator rows have node == nil and are not cursor targets.
type treeRow struct {
	node      *NavNode
	depth     int
	path      []int  // indexes from root to this node (nil for separators)
	expanded  bool   // meaningful for groups only
	separator bool   // blank gap row before a special group (Options)
	digit     int    // global top-level ordinal 1–9 for digit jump hints; 0 = none
	branch    string // tree-command prefix for nested rows (e.g. "│   ├── ")
}

// visibleRows walks nav depth-first, emitting only expanded branches.
// gapBefore on a node inserts a non-selectable separator row above it.
// digit is assigned from the global top-level order before any windowing.
func visibleRows(nav []NavNode, expanded map[string]bool) []treeRow {
	var out []treeRow
	topDigit := 0
	var walk func(nodes []NavNode, depth int, prefix []int, treePrefix string)
	walk = func(nodes []NavNode, depth int, prefix []int, treePrefix string) {
		for i := range nodes {
			path := append(append([]int{}, prefix...), i)
			n := &nodes[i]
			if n.GapBefore {
				out = append(out, treeRow{separator: true, depth: depth})
			}
			key := pathKey(path)
			isGroup := n.IsGroup()
			open := isGroup && expanded[key]
			digit := 0
			branch := ""
			if depth == 0 {
				topDigit++
				if topDigit <= 9 {
					digit = topDigit
				}
			} else {
				conn := "├── "
				if i == len(nodes)-1 {
					conn = "└── "
				}
				branch = treePrefix + conn
			}
			out = append(out, treeRow{
				node:     n,
				depth:    depth,
				path:     path,
				expanded: open,
				digit:    digit,
				branch:   branch,
			})
			if open {
				childPrefix := treePrefix
				if depth == 0 {
					childPrefix = ""
				} else if i == len(nodes)-1 {
					childPrefix = treePrefix + "    "
				} else {
					childPrefix = treePrefix + "│   "
				}
				walk(n.Children, depth+1, path, childPrefix)
			}
		}
	}
	walk(nav, 0, nil, "")
	return out
}

// windowRows returns a slice of rows that fits height, scrolled so cursorIdx
// stays visible. Separators count toward height like any other row.
func windowRows(rows []treeRow, cursorIdx, height int) (window []treeRow, offset int) {
	if height <= 0 || len(rows) == 0 {
		return nil, 0
	}
	if len(rows) <= height {
		return rows, 0
	}
	if cursorIdx < 0 {
		cursorIdx = 0
	}
	if cursorIdx >= len(rows) {
		cursorIdx = len(rows) - 1
	}
	offset = cursorIdx - height/2
	if offset < 0 {
		offset = 0
	}
	if offset+height > len(rows) {
		offset = len(rows) - height
	}
	return rows[offset : offset+height], offset
}

// selectableRowIndex maps a cursor path onto the nearest selectable row index
// in rows (skips separators).
func selectableRowIndex(rows []treeRow, path []int) int {
	if len(path) == 0 {
		for i, r := range rows {
			if !r.separator {
				return i
			}
		}
		return 0
	}
	key := pathKey(path)
	for i, r := range rows {
		if !r.separator && pathKey(r.path) == key {
			return i
		}
	}
	best := 0
	bestLen := -1
	for i, r := range rows {
		if r.separator {
			continue
		}
		n := commonPrefixLen(r.path, path)
		if n > bestLen {
			bestLen = n
			best = i
		}
	}
	return best
}

// stepSelectable moves delta selectable steps from fromIdx, skipping separators.
// fromIdx may be -1 (before first) or len(rows) (after last).
func stepSelectable(rows []treeRow, fromIdx, delta int) int {
	if len(rows) == 0 {
		return 0
	}
	dir := 1
	if delta < 0 {
		dir = -1
		delta = -delta
	}
	i := fromIdx
	for step := 0; step < delta; step++ {
		next := i + dir
		for next >= 0 && next < len(rows) && rows[next].separator {
			next += dir
		}
		if next < 0 || next >= len(rows) {
			if i >= 0 && i < len(rows) && !rows[i].separator {
				return i
			}
			for j := range rows {
				idx := j
				if dir < 0 {
					idx = len(rows) - 1 - j
				}
				if !rows[idx].separator {
					return idx
				}
			}
			return 0
		}
		i = next
	}
	if i < 0 || i >= len(rows) || rows[i].separator {
		return selectableRowIndex(rows, nil)
	}
	return i
}

func pathKey(path []int) string {
	if len(path) == 0 {
		return ""
	}
	b := make([]byte, 0, len(path)*2)
	for i, p := range path {
		if i > 0 {
			b = append(b, '.')
		}
		if p == 0 {
			b = append(b, '0')
			continue
		}
		var tmp [12]byte
		n := 0
		for x := p; x > 0; x /= 10 {
			tmp[n] = byte('0' + x%10)
			n++
		}
		for j := n - 1; j >= 0; j-- {
			b = append(b, tmp[j])
		}
	}
	return string(b)
}

func nodeAt(nav []NavNode, path []int) *NavNode {
	nodes := nav
	var cur *NavNode
	for _, idx := range path {
		if idx < 0 || idx >= len(nodes) {
			return nil
		}
		cur = &nodes[idx]
		nodes = cur.Children
	}
	return cur
}

// firstLeafPath returns the path of the first leaf under path (inclusive).
func firstLeafPath(nav []NavNode, path []int) []int {
	n := nodeAt(nav, path)
	if n == nil {
		return nil
	}
	if !n.IsGroup() {
		return path
	}
	if len(n.Children) == 0 {
		return nil
	}
	child := append(append([]int{}, path...), 0)
	return firstLeafPath(nav, child)
}

// defaultExpanded opens every top-level group so Options children are visible
// on first paint; nested groups stay collapsed.
func defaultExpanded(nav []NavNode) map[string]bool {
	out := make(map[string]bool)
	for i := range nav {
		if nav[i].IsGroup() {
			out[pathKey([]int{i})] = true
		}
	}
	return out
}

// ensureLeafActive picks a leaf path for content when path is empty/group.
func ensureLeafActive(nav []NavNode, path []int) []int {
	if len(nav) == 0 {
		return nil
	}
	if len(path) == 0 {
		return firstLeafPath(nav, []int{0})
	}
	n := nodeAt(nav, path)
	if n == nil {
		return firstLeafPath(nav, []int{0})
	}
	if n.IsGroup() {
		return firstLeafPath(nav, path)
	}
	return path
}
