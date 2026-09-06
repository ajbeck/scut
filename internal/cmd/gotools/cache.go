//go:build goexperiment.jsonv2

package gotools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	json "encoding/json/v2"

	"github.com/ajbeck/scut/internal/godoc"
)

type cacheCmd struct {
	Path   cachePathCmd   `cmd:"path" help:"Print the scut module cache path."`
	List   cacheListCmd   `cmd:"list" help:"List scut module cache entries."`
	Verify cacheVerifyCmd `cmd:"verify" help:"Verify scut module cache entries."`
	Remove cacheRemoveCmd `cmd:"remove" help:"Remove one module or exact module version."`
	Clean  cacheCleanCmd  `cmd:"clean" help:"Remove every scut module cache entry."`
	Prune  cachePruneCmd  `cmd:"prune" help:"Prune scut module cache entries by age or size."`
}

type cachePathCmd struct{}

type cacheListCmd struct {
	Target string `arg:"" optional:"" name:"module" help:"Optional module or module@version filter." placeholder:"MODULE[@VERSION]"`
	JSON   bool   `help:"Emit a structured JSON object." name:"json"`
}

type cacheVerifyCmd struct {
	Target string `arg:"" optional:"" name:"module" help:"Optional module or module@version filter." placeholder:"MODULE[@VERSION]"`
	JSON   bool   `help:"Emit a structured JSON object." name:"json"`
}

type cacheRemoveCmd struct {
	Target string `arg:"" name:"module" help:"Module or exact module version to remove." placeholder:"MODULE[@VERSION]"`
	JSON   bool   `help:"Emit a structured JSON object." name:"json"`
}

type cacheCleanCmd struct {
	JSON bool `help:"Emit a structured JSON object." name:"json"`
}

type cachePruneCmd struct {
	OlderThan string `help:"Remove entries older than this duration (for example 720h)." name:"older-than" placeholder:"DURATION"`
	MaxSize   string `help:"Reduce the cache to at most this size (for example 1GiB)." name:"max-size" placeholder:"SIZE"`
	JSON      bool   `help:"Emit a structured JSON object." name:"json"`
	now       func() time.Time
}

type cacheEntryOutput struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Revision string `json:"revision,omitzero"`
	Latest   bool   `json:"latest,omitzero"`
	Status   string `json:"status"`
	Problem  string `json:"problem,omitzero"`
}

type cacheProblemOutput struct {
	Path    string `json:"path"`
	Problem string `json:"problem"`
}

type cacheListOutput struct {
	Path     string               `json:"path"`
	Size     int64                `json:"size"`
	Entries  []cacheEntryOutput   `json:"entries"`
	Problems []cacheProblemOutput `json:"problems"`
}

type cacheVerifyOutput struct {
	Path     string               `json:"path"`
	Checked  int                  `json:"checked"`
	Valid    bool                 `json:"valid"`
	Problems []cacheProblemOutput `json:"problems"`
}

type cacheMutationOutput struct {
	Path       string `json:"path"`
	Removed    int    `json:"removed"`
	BytesFreed int64  `json:"bytesFreed"`
}

type cacheVerificationError struct {
	Problems int
}

func (e cacheVerificationError) Error() string {
	return fmt.Sprintf("module cache verification found %d problem(s)", e.Problems)
}

var newModuleCacheStore = func() (godoc.FileArchiveStore, error) {
	root, err := godoc.DefaultModuleArchiveCacheDir()
	if err != nil {
		return godoc.FileArchiveStore{}, err
	}
	return godoc.FileArchiveStore{Root: root}, nil
}

func (c *cachePathCmd) Run(stdout io.Writer) error {
	store, err := newModuleCacheStore()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, store.Root)
	return err
}

func (c *cacheListCmd) Run(stdout io.Writer) error {
	_, inventory, err := inspectModuleCache(c.Target)
	if err != nil {
		return err
	}
	if c.JSON {
		return writeCacheJSON(stdout, listOutput(inventory))
	}
	return writeCacheList(stdout, inventory)
}

func (c *cacheVerifyCmd) Run(stdout io.Writer) error {
	_, inventory, err := inspectModuleCache(c.Target)
	if err != nil {
		return err
	}
	problems := verificationProblems(inventory)
	if c.JSON {
		err = writeCacheJSON(stdout, cacheVerifyOutput{
			Path:     inventory.Root,
			Checked:  len(inventory.Entries),
			Valid:    len(problems) == 0,
			Problems: problems,
		})
	} else {
		err = writeCacheVerification(stdout, inventory, problems)
	}
	if err != nil {
		return err
	}
	if len(problems) != 0 {
		return cacheVerificationError{Problems: len(problems)}
	}
	return nil
}

func (c *cacheRemoveCmd) Run(stdout io.Writer) error {
	target, err := godoc.ParseModuleCacheTarget(c.Target)
	if err != nil {
		return err
	}
	store, err := newModuleCacheStore()
	if err != nil {
		return err
	}
	removed, err := store.RemoveCache(context.Background(), target)
	if err != nil {
		return err
	}
	return writeCacheMutation(stdout, store.Root, removed, c.JSON)
}

func (c *cacheCleanCmd) Run(stdout io.Writer) error {
	store, err := newModuleCacheStore()
	if err != nil {
		return err
	}
	removed, err := store.CleanCache(context.Background())
	if err != nil {
		return err
	}
	return writeCacheMutation(stdout, store.Root, removed, c.JSON)
}

func (c *cachePruneCmd) Run(stdout io.Writer) error {
	policy, err := c.policy()
	if err != nil {
		return err
	}
	store, err := newModuleCacheStore()
	if err != nil {
		return err
	}
	removed, err := store.PruneCache(context.Background(), policy)
	writeErr := writeCacheMutation(stdout, store.Root, removed, c.JSON)
	if writeErr != nil {
		return writeErr
	}
	return err
}

func (c *cachePruneCmd) policy() (godoc.ModuleCachePrunePolicy, error) {
	policy := godoc.ModuleCachePrunePolicy{}
	if c.OlderThan != "" {
		olderThan, err := time.ParseDuration(c.OlderThan)
		if err != nil {
			return policy, fmt.Errorf("invalid older-than duration %q: %w", c.OlderThan, err)
		}
		policy.OlderThan = &olderThan
	}
	if c.MaxSize != "" {
		maxSize, err := parseByteSize(c.MaxSize)
		if err != nil {
			return policy, err
		}
		policy.MaxSize = &maxSize
	}
	if policy.OlderThan == nil && policy.MaxSize == nil {
		return policy, errors.New("cache prune requires --older-than or --max-size")
	}
	if c.now != nil {
		policy.Now = c.now()
	}
	return policy, nil
}

func inspectModuleCache(rawTarget string) (godoc.FileArchiveStore, godoc.ModuleCacheInventory, error) {
	store, err := newModuleCacheStore()
	if err != nil {
		return store, godoc.ModuleCacheInventory{}, err
	}
	inventory, err := store.InspectCache(context.Background())
	if err != nil {
		return store, inventory, err
	}
	if rawTarget == "" {
		return store, inventory, nil
	}
	target, err := godoc.ParseModuleCacheTarget(rawTarget)
	if err != nil {
		return store, inventory, err
	}
	inventory = inventory.Filter(target)
	if len(inventory.Entries) == 0 && len(inventory.Problems) == 0 {
		return store, inventory, fmt.Errorf("%w: %s", godoc.ErrArchiveNotFound, rawTarget)
	}
	return store, inventory, nil
}

func listOutput(inventory godoc.ModuleCacheInventory) cacheListOutput {
	out := cacheListOutput{
		Path:     inventory.Root,
		Size:     inventory.Size,
		Entries:  make([]cacheEntryOutput, 0, len(inventory.Entries)),
		Problems: make([]cacheProblemOutput, 0, len(inventory.Problems)),
	}
	for _, entry := range inventory.Entries {
		out.Entries = append(out.Entries, cacheEntryOutput{
			Module:   entry.Module.Path,
			Version:  entry.Module.Version,
			Path:     entry.Path,
			Size:     entry.Size,
			Modified: entry.Modified.UTC().Format(time.RFC3339Nano),
			Revision: entry.Revision,
			Latest:   entry.Latest,
			Status:   string(entry.Status),
			Problem:  entry.Problem,
		})
	}
	for _, problem := range inventory.Problems {
		out.Problems = append(out.Problems, cacheProblemOutput(problem))
	}
	return out
}

func verificationProblems(inventory godoc.ModuleCacheInventory) []cacheProblemOutput {
	problems := make([]cacheProblemOutput, 0, len(inventory.Problems)+len(inventory.Entries))
	for _, entry := range inventory.Entries {
		if entry.Status != godoc.ModuleCacheEntryValid {
			problems = append(problems, cacheProblemOutput{Path: entry.Path, Problem: entry.Problem})
		}
	}
	for _, problem := range inventory.Problems {
		problems = append(problems, cacheProblemOutput(problem))
	}
	return problems
}

func writeCacheList(stdout io.Writer, inventory godoc.ModuleCacheInventory) error {
	if len(inventory.Entries) == 0 && len(inventory.Problems) == 0 {
		_, err := fmt.Fprintln(stdout, "cache is empty")
		return err
	}
	w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "MODULE\tVERSION\tSIZE\tMODIFIED\tSTATUS\tLATEST"); err != nil {
		return err
	}
	for _, entry := range inventory.Entries {
		latest := ""
		if entry.Latest {
			latest = "yes"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.Module.Path,
			entry.Module.Version,
			formatByteSize(entry.Size),
			entry.Modified.UTC().Format(time.RFC3339),
			entry.Status,
			latest,
		); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	for _, problem := range inventory.Problems {
		if _, err := fmt.Fprintf(stdout, "problem: %s: %s\n", problem.Path, problem.Problem); err != nil {
			return err
		}
	}
	for _, entry := range inventory.Entries {
		if entry.Problem == "" {
			continue
		}
		if _, err := fmt.Fprintf(stdout, "problem: %s: %s\n", entry.Path, entry.Problem); err != nil {
			return err
		}
	}
	return nil
}

func writeCacheVerification(stdout io.Writer, inventory godoc.ModuleCacheInventory, problems []cacheProblemOutput) error {
	for _, entry := range inventory.Entries {
		if entry.Status == godoc.ModuleCacheEntryValid {
			if _, err := fmt.Fprintf(stdout, "ok: %s@%s\n", entry.Module.Path, entry.Module.Version); err != nil {
				return err
			}
		}
	}
	for _, problem := range problems {
		if _, err := fmt.Fprintf(stdout, "problem: %s: %s\n", problem.Path, problem.Problem); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(stdout, "checked %s; found %s\n", pluralEntries(len(inventory.Entries)), pluralProblems(len(problems)))
	return err
}

func writeCacheMutation(stdout io.Writer, root string, removal godoc.ModuleCacheRemoval, jsonOutput bool) error {
	if jsonOutput {
		return writeCacheJSON(stdout, cacheMutationOutput{
			Path:       root,
			Removed:    removal.Entries,
			BytesFreed: removal.Bytes,
		})
	}
	_, err := fmt.Fprintf(stdout, "removed %s, freeing %s\n", pluralEntries(removal.Entries), formatByteSize(removal.Bytes))
	return err
}

func pluralEntries(count int) string {
	if count == 1 {
		return "1 module cache entry"
	}
	return fmt.Sprintf("%d module cache entries", count)
}

func pluralProblems(count int) string {
	if count == 1 {
		return "1 problem"
	}
	return fmt.Sprintf("%d problems", count)
}

func writeCacheJSON(stdout io.Writer, value any) error {
	data, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = stdout.Write(data)
	return err
}

func parseByteSize(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("max-size is empty")
	}
	index := 0
	for index < len(value) && ((value[index] >= '0' && value[index] <= '9') || value[index] == '.') {
		index++
	}
	number, unit := value[:index], strings.ToUpper(strings.TrimSpace(value[index:]))
	if number == "" {
		return 0, fmt.Errorf("invalid max-size %q", raw)
	}
	amount, err := strconv.ParseFloat(number, 64)
	if err != nil || amount < 0 {
		return 0, fmt.Errorf("invalid max-size %q", raw)
	}
	multipliers := map[string]float64{
		"": 1, "B": 1,
		"KB": 1_000, "MB": 1_000_000, "GB": 1_000_000_000, "TB": 1_000_000_000_000,
		"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40,
	}
	multiplier, ok := multipliers[unit]
	if !ok || math.IsInf(amount, 0) || amount*multiplier > math.MaxInt64 {
		return 0, fmt.Errorf("invalid max-size %q", raw)
	}
	return int64(math.Round(amount * multiplier)), nil
}

func formatByteSize(size int64) string {
	const unit = int64(1024)
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	divisor, suffix := unit, "KiB"
	for _, candidate := range []struct {
		divisor int64
		suffix  string
	}{{unit * unit, "MiB"}, {unit * unit * unit, "GiB"}, {unit * unit * unit * unit, "TiB"}} {
		if size < candidate.divisor {
			break
		}
		divisor, suffix = candidate.divisor, candidate.suffix
	}
	return fmt.Sprintf("%.1f%s", float64(size)/float64(divisor), suffix)
}
