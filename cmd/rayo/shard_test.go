package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/nanihadesuka/rayo/internal/flow"
)

func TestShardParse(t *testing.T) {
	for v, ok := range map[string]bool{
		"1/1": true, "2/4": true, "4/4": true,
		"1x/2": false, "1/2x": false, "1/2/3": false, "0/2": false, "3/2": false, "1/0": false, "2": false, "/2": false, "1/": false, "": false, "a/b": false,
	} {
		o, err := parse(cmdTest, []string{"--shard", v})
		if (err == nil) != ok {
			t.Errorf("--shard %q: %v", v, err)
		}
		if ok && (o.shard < 1 || o.shards < o.shard) {
			t.Errorf("--shard %q: %d/%d", v, o.shard, o.shards)
		}
	}
}

// Every flow is in exactly one part, whatever the order they come in.
func TestShardParts(t *testing.T) {
	var flows []*flow.Flow
	for i := range 23 {
		flows = append(flows, &flow.Flow{Name: fmt.Sprint(i), Path: fmt.Sprintf("f%02d.yaml", i)})
	}
	for _, n := range []int{1, 2, 3, 4, 7, 30} {
		seen := map[*flow.Flow]int{}
		for i := 1; i <= n; i++ {
			o := &options{shard: i, shards: n}
			mine := o.part(slices.Clone(flows))
			reversed := slices.Clone(flows)
			slices.Reverse(reversed)
			again := o.part(reversed)
			slices.Reverse(again)
			if !slices.Equal(mine, again) {
				t.Errorf("part %d of %d depends on the order", i, n)
			}
			if len(mine) > (len(flows)+n-1)/n {
				t.Errorf("part %d of %d has %d flows", i, n, len(mine))
			}
			for _, f := range mine {
				seen[f]++
			}
		}
		for _, f := range flows {
			if seen[f] != 1 {
				t.Errorf("of %d parts, %s is in %d", n, f.Path, seen[f])
			}
		}
	}
}
