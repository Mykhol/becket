package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show local command usage statistics",
		Args:  cobra.NoArgs,
		Run:   func(_ *cobra.Command, _ []string) { runStats() },
	}
}

type usageEntry struct {
	Cmd       string `json:"cmd"`
	Workspace string `json:"workspace"`
	Platform  string `json:"platform"`
}

func runStats() {
	path := usageLogPath()
	f, err := os.Open(path)
	if err != nil {
		fmt.Println("No usage data yet. Run some becket commands to start collecting stats.")
		return
	}
	defer f.Close()

	var entries []usageEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e usageEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		fmt.Println("No usage data yet.")
		return
	}

	cmds := newCounter()
	wss := newCounter()
	plats := newCounter()
	for _, e := range entries {
		cmds.add(e.Cmd)
		if e.Workspace != "" {
			wss.add(e.Workspace)
		}
		if e.Platform != "" {
			plats.add(e.Platform)
		}
	}

	fmt.Printf("Total commands run: %d\n", len(entries))
	fmt.Println()
	fmt.Println("Command usage:")
	for _, kv := range cmds.mostCommon(0) {
		bar := strings.Repeat("█", min(kv.count, 30))
		fmt.Printf("  %-15s %5d  %s\n", kv.key, kv.count, bar)
	}
	if wss.total() > 0 {
		fmt.Println()
		fmt.Println("Most active workspaces:")
		for _, kv := range wss.mostCommon(5) {
			fmt.Printf("  %-25s %5d\n", kv.key, kv.count)
		}
	}
	if plats.total() > 0 {
		fmt.Println()
		fmt.Println("Platforms:")
		for _, kv := range plats.mostCommon(0) {
			fmt.Printf("  %-25s %5d\n", kv.key, kv.count)
		}
	}
}

// counter preserves first-seen order so ties resolve like Python's Counter.
type counter struct {
	order  []string
	counts map[string]int
}

type kv struct {
	key   string
	count int
}

func newCounter() *counter { return &counter{counts: map[string]int{}} }

func (c *counter) add(k string) {
	if _, ok := c.counts[k]; !ok {
		c.order = append(c.order, k)
	}
	c.counts[k]++
}

func (c *counter) total() int { return len(c.order) }

// mostCommon returns entries by count desc, ties in insertion order; limit<=0
// means all.
func (c *counter) mostCommon(limit int) []kv {
	out := make([]kv, 0, len(c.order))
	for _, k := range c.order {
		out = append(out, kv{k, c.counts[k]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].count > out[j].count })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
