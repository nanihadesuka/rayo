package schema

import (
	"slices"
	"strings"
)

// synonyms are words someone new may write for a name of rayo's, from other tools or plain English.
var synonyms = map[string][]string{
	"folder":  {"dir", "directory", "path", "outdir", "output-dir", "outputdir"},
	"video":   {"record", "recording", "videos", "screenrecord"},
	"results": {"junit", "report", "reports", "result"},
	"retries": {"retry", "rerun", "reruns", "attempts"},
	"env":     {"vars", "variables", "environment", "var"},
	"keep":    {"retain", "keep-files"},
}

// Nearest is the name most like one that isn't one: the same but for case, a word of the same
// meaning, a few letters away, or the only one it begins like; empty if none is close enough to be
// what was meant. A leading - is ignored, for options.
func Nearest(name string, names []string) string {
	bare := func(s string) string { return strings.ToLower(strings.TrimLeft(s, "-")) }
	want := bare(name)
	for _, n := range names {
		if bare(n) == want {
			return n
		}
	}
	for _, n := range names {
		if slices.Contains(synonyms[bare(n)], want) {
			return n
		}
	}
	best, bestD := "", 0
	for _, n := range names {
		d := Distance([]rune(want), []rune(bare(n)))
		if d <= max(1, len([]rune(bare(n)))/3) && (best == "" || d < bestD) {
			best, bestD = n, d
		}
	}
	if best != "" {
		return best
	}
	// the same beginning, of at least 4 letters, that only one name has: retry, retries
	var alike []string
	for _, n := range names {
		b, i := bare(n), 0
		for i < len(b) && i < len(want) && b[i] == want[i] {
			i++
		}
		if i >= 4 && i*3 >= len(want)*2 {
			alike = append(alike, n)
		}
	}
	if len(alike) == 1 {
		return alike[0]
	}
	return ""
}

// Distance is Levenshtein's, with two letters swapped counted as one change, the commonest slip: tset, test.
func Distance(a, b []rune) int {
	before, last, row := make([]int, len(b)+1), make([]int, len(b)+1), make([]int, len(b)+1)
	for j := range last {
		last[j] = j
	}
	for i := 1; i <= len(a); i++ {
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			row[j] = min(last[j]+1, row[j-1]+1, last[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				row[j] = min(row[j], before[j-2]+1)
			}
		}
		before, last, row = last, row, before
	}
	return last[len(b)]
}
