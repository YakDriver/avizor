# AGENTS.md

Avizor gathers evidence for a decision; it never decides or writes to GitHub. Keep that line: observed evidence, labeled inferences, and candidates stay separate in output, and every inference states its basis.

- **Measure, don't guess.** `avizor eval` is the check for any change to retrieval, categorization, or queue states. Run it before and after. Report changes in cases passed and precision, and label new confident candidates (see [eval/README.md](eval/README.md)).
- **Refactors and speedups change nothing.** Eval case results and `queue --all` output must be identical before and after. Compare them.
- **Speed matters.** A faster queue, inspect, or eval means faster decisions. It ranks below correctness, but measure the time a change adds, and profile before optimizing.
- **Don't touch real state.** Run experiments against a copy (`sqlite3 ... ".backup"`, then `--state-dir`), never `~/.local/share/avizor`.
- **Deterministic output.** Sorts end in a unique tie-break. Concurrent work reports results in input order.
- **Concurrency.** Queue and eval run jobs in parallel. Shared state must be safe for that, so test with `-race`.
- `make ci` before committing. Tests need ripgrep and git.
- Docs and PRs: plain, short sentences with real numbers. PRs have Changes and Results sections and end with the AI-assistance note.
- Public issues and comments, on any repo: terse, natural language. No meta-narration, prompt leakage, needless explanation, or hedging.
- Public issues should be tidy: streamlined, terse, and natural, with the minimum needed to convey the necessary information.
- A missing prerequisite (an SDK version, a resource, or a service in the target) isn't a reason to defer an issue. An issue gathers interest while the prerequisite is worked out. Defer only when there's no API yet.
