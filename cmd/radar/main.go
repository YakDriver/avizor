// Command radar finds what is already known about new information and gathers evidence for a human or agent to act on.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/YakDriver/radar/internal/awssdk"
	"github.com/YakDriver/radar/internal/config"
	"github.com/YakDriver/radar/internal/github"
	"github.com/YakDriver/radar/internal/index"
	"github.com/YakDriver/radar/internal/reconcile"
	"github.com/YakDriver/radar/internal/services"
	"github.com/YakDriver/radar/internal/syncer"
)

var version = "dev"

const usage = `radar finds existing issues, pull requests, and code related to new information.

Usage:
  radar sync [flags]                         Refresh the local index from GitHub.
  radar candidates [flags] <issue|url>       Reconcile an issue number, issue URL, or other URL.
  radar candidates [flags] --text "..."      Reconcile a free-text description.
  radar eval [flags] [case...]               Run the evaluation corpus.
  radar pull [flags] [source]                Pull new items from a source (default: aws-sdk-go-v2).
  radar items [flags]                        List pulled items.
  radar inspect [flags] <item-id>            Show an item's evidence.
  radar version

Common flags:
  --config path      Config file (default ./radar.hcl, then $XDG_CONFIG_HOME/radar/radar.hcl)
  --state-dir path   State directory (default $XDG_DATA_HOME/radar)
  --target name      Target to use when several are configured
  --format fmt       Output format: json (default) or text

Run "radar <command> -h" for command flags.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "radar:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

var errUsage = errors.New("usage error")

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errUsage
	}
	switch args[0] {
	case "sync":
		return cmdSync(ctx, args[1:], stdout, stderr)
	case "candidates":
		return cmdCandidates(ctx, args[1:], stdout, stderr)
	case "eval":
		return cmdEval(ctx, args[1:], stdout, stderr)
	case "pull":
		return cmdPull(ctx, args[1:], stdout, stderr)
	case "items":
		return cmdItems(ctx, args[1:], stdout, stderr)
	case "inspect":
		return cmdInspect(ctx, args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "radar", version)
		return nil
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("%w: unknown command %q", errUsage, args[0])
	}
}

// common holds flags shared by commands.
type common struct {
	configPath string
	stateDir   string
	target     string
	format     string

	// Set by open.
	cfg *config.Config
	dir string
}

func (c *common) register(fs *flag.FlagSet) {
	fs.StringVar(&c.configPath, "config", "", "config file")
	fs.StringVar(&c.stateDir, "state-dir", "", "state directory")
	fs.StringVar(&c.target, "target", "", "target name")
	fs.StringVar(&c.format, "format", "json", "output format: json or text")
}

// load reads the config and resolves the state directory: --state-dir, then the config's state_dir, then the default.
func (c *common) load() (*config.Config, string, error) {
	if c.format != "json" && c.format != "text" {
		return nil, "", fmt.Errorf("%w: --format must be json or text", errUsage)
	}
	path := c.configPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return nil, "", err
		}
		path = p
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	dir := cmp.Or(c.stateDir, cfg.StateDir)
	if dir == "" {
		if dir, err = config.DefaultStateDir(); err != nil {
			return nil, "", err
		}
	}
	return cfg, dir, nil
}

// open loads config and opens the target's index.
func (c *common) open() (*config.Target, *index.Store, error) {
	cfg, dir, err := c.load()
	if err != nil {
		return nil, nil, err
	}
	t, err := cfg.Target(c.target)
	if err != nil {
		return nil, nil, err
	}
	if err := config.MigrateIndex(dir, t.Name); err != nil {
		return nil, nil, err
	}
	st, err := index.Open(config.IndexPath(dir, t.Name))
	if err != nil {
		return nil, nil, err
	}
	c.cfg, c.dir = cfg, dir
	return t, st, nil
}

// operations sets r's operation names from the target's models source, if it has one and its clone exists. Without
// them, any verb-prefixed PascalCase word counts as an operation, so a missing clone is a warning.
func (c *common) operations(ctx context.Context, t *config.Target, r *reconcile.Reconciler, stderr io.Writer) {
	if t.Operations == config.OperationsNone {
		return
	}
	src, err := c.cfg.Source(t.Operations)
	if err != nil {
		fmt.Fprintf(stderr, "radar: warning: operation names unavailable: %v\n", err)
		return
	}
	dir := src.ModelsPath(c.dir)
	if _, err := os.Stat(dir); err != nil {
		fmt.Fprintf(stderr, "radar: warning: operation names unavailable: no models clone at %s; run `radar pull %s`\n", dir, src.Name)
		return
	}
	repo := &awssdk.ModelsRepo{Dir: dir, URL: src.ModelsURL, Branch: src.ModelsBranch}
	ops, err := repo.Operations(ctx, dir+".operations.json")
	if err != nil {
		fmt.Fprintf(stderr, "radar: warning: operation names unavailable: %v\n", err)
		return
	}
	r.Operations = ops
	r.OperationsSource = fmt.Sprintf("%s@%s (%d operations)", src.ModelsURL, ops.Commit, len(ops.Names))
}

// parseInterspersed parses flags that may appear before or after positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		if args[0] == "--" {
			return append(pos, args[1:]...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func cmdSync(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	full := fs.Bool("full", false, "refetch everything and remove records GitHub no longer returns")
	sinceFlag := fs.String("since", "", "only fetch records updated since a date (2026-01-02), time (RFC 3339), or age (90d, 12h); doesn't move checkpoints")
	quiet := fs.Bool("quiet", false, "suppress progress messages")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 0 {
		return fmt.Errorf("%w: sync takes no arguments", errUsage)
	}
	if *full && *sinceFlag != "" {
		return fmt.Errorf("%w: --full and --since can't be combined", errUsage)
	}
	var since time.Time
	if *sinceFlag != "" {
		if since, err = parseSince(*sinceFlag, time.Now()); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()

	gh, err := github.NewClient(ctx)
	if err != nil {
		return err
	}
	logf := func(format string, args ...any) {
		if !*quiet {
			fmt.Fprintf(stderr, format+"\n", args...)
		}
	}
	gh.Log = logf

	res, err := syncer.Run(ctx, gh, st, t, syncer.Options{Full: *full, Since: since, Log: logf})
	if err != nil {
		if res != nil {
			logf("partial sync: %d issues, %d comments before the error; progress is saved", res.Issues, res.Comments)
		}
		return err
	}

	if c.format == "text" {
		fmt.Fprintf(stdout, "synced %s: %d issues and PRs, %d comments, %d removed in %s\n", res.Repo, res.Issues, res.Comments, res.Removed, res.Duration)
		return nil
	}
	return writeJSON(stdout, res)
}

func parseSince(s string, now time.Time) (time.Time, error) {
	t, err := config.ParseSince(s, now)
	if err != nil {
		return time.Time{}, fmt.Errorf("--since: %w", err)
	}
	return t, nil
}

func cmdCandidates(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("candidates", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	text := fs.String("text", "", "free-text description to reconcile")
	noFetch := fs.Bool("no-fetch", false, "don't fetch URL inputs to read their title")
	noCode := fs.Bool("no-code", false, "don't search the target checkout")
	related := fs.Int("related", 0, "maximum related candidates (default 10)")
	asOfFlag := fs.String("as-of", "", "reconcile against the index as it was at a date (2026-01-02, end of day) or RFC 3339 time")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(pos) > 1 {
		return fmt.Errorf("%w: candidates takes one argument", errUsage)
	}
	var arg string
	if len(pos) == 1 {
		arg = pos[0]
	}
	var asOf time.Time
	if *asOfFlag != "" {
		if asOf, err = parseAsOf(*asOfFlag); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}

	t, st, err := c.open()
	if err != nil {
		return err
	}
	defer st.Close()

	r := &reconcile.Reconciler{
		Store:    st,
		Target:   t,
		Services: loadServices(t, stderr),
		HTTP:     defaultHTTP(),
		AsOf:     asOf,
		Options: reconcile.Options{
			NoFetch:      *noFetch,
			NoCode:       *noCode,
			RelatedLimit: *related,
			Version:      version,
		},
	}
	c.operations(ctx, t, r, stderr)
	if gh, err := github.NewClient(ctx); err == nil {
		r.Issues = fetcher{gh}
	}

	res, err := r.Run(ctx, arg, *text)
	if err != nil {
		return err
	}
	if c.format == "text" {
		return writeText(stdout, res)
	}
	return writeJSON(stdout, res)
}

func defaultHTTP() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func loadServices(t *config.Target, stderr io.Writer) *services.Map {
	p := t.NamesDataPath()
	if p == "" {
		return nil
	}
	svcs, err := services.Load(p)
	if err != nil {
		fmt.Fprintf(stderr, "radar: warning: service map unavailable: %v\n", err)
		return nil
	}
	return svcs
}

func parseAsOf(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t.Add(24*time.Hour - time.Second), nil
	}
	return time.Time{}, fmt.Errorf("invalid --as-of %q: use a date (2026-01-02) or RFC 3339 time", s)
}

// fetcher adapts the GitHub client to reconcile.IssueFetcher.
type fetcher struct {
	gh *github.Client
}

func (f fetcher) FetchIssue(ctx context.Context, repo string, n int) (*index.Issue, error) {
	is, err := f.gh.GetIssue(ctx, repo, n)
	if err != nil {
		return nil, err
	}
	return syncer.Convert(is), nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
