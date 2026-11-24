package schema

import "strings"

// Nearest is the name most like one that isn't one: the same but for case, or a few letters away;
// empty if none is close enough to be what was meant.
func Nearest(name string, names []string) string {
	best, bestD := "", 0
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return n
		}
		d := Distance([]rune(strings.ToLower(name)), []rune(strings.ToLower(n)))
		if d <= max(1, len([]rune(n))/3) && (best == "" || d < bestD) {
			best, bestD = n, d
		}
	}
	return best
}

// Distance is Levenshtein's.
func Distance(a, b []rune) int {
	row := make([]int, len(b)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(a); i++ {
		prev := row[0]
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cur := row[j]
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			row[j] = min(row[j]+1, row[j-1]+1, prev+cost)
			prev = cur
		}
	}
	return row[len(b)]
}
