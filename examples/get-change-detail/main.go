// Command get-change-detail reads a change with detail (GET /changes/{id} ->
// ChangeInfo, anonymous) and prints a colored, Web-UI-style summary using Gerrit's own
// palette -- the Go twin of the Rust example. Every value comes from the generated
// gerrit_client models; only the formatting is hand-written.
//
// The Gerrit XSSI guard ()]}' prefix on every JSON body) is stripped by the
// gerritxssi transport, the one Gerrit-specific step not expressible in OpenAPI.
//
//	go run github.com/davido/gerrit-sdk-go/examples/get-change-detail@latest -- --change 621763
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	gc "github.com/davido/gerrit-sdk-go/gerritclient"
	"github.com/davido/gerrit-sdk-go/gerritxssi"
)

// ListChangesOption values that populate every panel below from a single GET.
var options = []string{
	"LABELS", "DETAILED_ACCOUNTS", "DETAILED_LABELS", "CURRENT_REVISION",
	"CURRENT_COMMIT", "CURRENT_FILES", "SUBMIT_REQUIREMENTS",
}

func main() {
	url := flag.String("url", "https://gerrit-review.googlesource.com", "Gerrit base URL")
	change := flag.String("change", "621763", "numeric id or project~branch~Change-Id")
	noColor := flag.Bool("no-color", false, "disable ANSI color")
	flag.Parse()
	useColor = computeColor(*noColor)

	base := strings.TrimRight(*url, "/")
	cfg := gc.NewConfiguration()
	cfg.Servers = gc.ServerConfigurations{{URL: base}}
	cfg.HTTPClient = gerritxssi.Client() // strip Gerrit's )]}' XSSI guard
	cfg.UserAgent = "gerrit-sdk-go-example"
	client := gc.NewAPIClient(cfg)

	ci, _, err := client.ChangesAPI.GetChangesChangeId(context.Background(), *change).
		O2(options).Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	printChangeDetails(base, ci)
}

// ---- presentation -------------------------------------------------------------

func printChangeDetails(base string, ci *gc.ChangeInfo) {
	fmt.Println(rule())
	fmt.Printf("  %s  %s\n", statusBadge(ci), sgr(fmt.Sprintf("#%d", ci.GetNumber()), bold))
	fmt.Printf("  %s\n", sgr(ci.GetSubject(), bold))
	fmt.Println(rule())
	fmt.Printf("  %s\n", fg(fmt.Sprintf("%s/c/%s/+/%d", base, ci.GetProject(), ci.GetNumber()), blue700))

	section("Change Info")
	row("Owner", account(ci.GetOwner()))
	if commit, ok := currentCommit(ci); ok {
		row("Author", person(commit.GetAuthor()))
		row("Committer", person(commit.GetCommitter()))
	}
	row("Repo | Branch", fmt.Sprintf("%s | %s", link(ci.GetProject()), link(ci.GetBranch())))
	row("Change-Id", link(ci.GetChangeId()))
	if t := ci.GetTopic(); t != "" {
		row("Topic", link(t))
	}
	if h := ci.GetHashtags(); len(h) > 0 {
		row("Hashtags", link(strings.Join(h, ", ")))
	}
	if flags := flagChips(ci); len(flags) > 0 {
		row("Flags", strings.Join(flags, "  "))
	}
	row("Strategy", pascal(string(ci.GetSubmitType())))
	if p := parentCommit(ci); p != "" {
		row("Parent", link(short(p)))
	}
	row("Patch set", fmt.Sprintf("%d", ci.GetCurrentRevisionNumber()))
	row("Updated", ci.GetUpdated())
	row("Size", plusminus(ci.GetInsertions(), ci.GetDeletions()))
	row("Comments", commentsSummary(ci))

	// Reviewers / CC -- one account per line (no comma overflow).
	for _, kt := range []struct{ key, title string }{{"REVIEWER", "Reviewers"}, {"CC", "CC"}} {
		people := ci.GetReviewers()[kt.key]
		if len(people) == 0 {
			continue
		}
		section(kt.title)
		for _, a := range people {
			fmt.Printf("    %s\n", account(a))
		}
	}

	if reqs := ci.GetSubmitRequirements(); len(reqs) > 0 {
		section("Submit Requirements")
		for _, r := range reqs {
			icon, text := reqParts(string(r.GetStatus()))
			fmt.Printf("    %s %-26s %s\n", icon, r.GetName(), text)
		}
	}

	if labels := ci.GetLabels(); len(labels) > 0 {
		section("Votes")
		names := make([]string, 0, len(labels))
		for n := range labels {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			li := labels[n] // map values aren't addressable; copy for the pointer method
			var chips []string
			for _, a := range li.GetAll() {
				if v := a.GetValue(); v != 0 { // only non-zero, like the UI aggregate
					chips = append(chips, voteChip(v, a.GetName()))
				}
			}
			value := sgr("—", dim)
			if len(chips) > 0 {
				value = strings.Join(chips, "  ")
			}
			fmt.Printf("    %-22s %s\n", n, value)
		}
	}

	if files := currentFiles(ci); len(files) > 0 {
		section(fmt.Sprintf("Files (patch set %d)", ci.GetCurrentRevisionNumber()))
		paths := make([]string, 0, len(files))
		for p := range files {
			paths = append(paths, p)
		}
		// Commit message pseudo-file first, then the rest alphabetically -- like the UI.
		sort.Slice(paths, func(i, j int) bool {
			ci, cj := paths[i] == "/COMMIT_MSG", paths[j] == "/COMMIT_MSG"
			if ci != cj {
				return ci
			}
			return strings.ToLower(paths[i]) < strings.ToLower(paths[j])
		})
		for _, p := range paths {
			f := files[p]
			letter, color := fileStatus(f.GetStatus())
			name := p
			switch {
			case p == "/COMMIT_MSG":
				name = "Commit message"
			case f.GetOldPath() != "":
				name = f.GetOldPath() + " → " + p
			}
			counts := plusminus(f.GetLinesInserted(), f.GetLinesDeleted())
			fmt.Printf("    %s %-52s %s\n", fg(letter, color), name, counts)
		}
	}

	fmt.Println(rule())
}

// ---- model accessors ----------------------------------------------------------

func currentCommit(ci *gc.ChangeInfo) (gc.CommitInfo, bool) {
	cr := ci.GetCurrentRevision()
	if cr == "" {
		return gc.CommitInfo{}, false
	}
	rev, ok := ci.GetRevisions()[cr]
	if !ok {
		return gc.CommitInfo{}, false
	}
	return rev.GetCommit(), true
}

func currentFiles(ci *gc.ChangeInfo) map[string]gc.CommonFileInfo {
	cr := ci.GetCurrentRevision()
	if cr == "" {
		return nil
	}
	rev, ok := ci.GetRevisions()[cr]
	if !ok {
		return nil
	}
	return rev.GetFiles()
}

func parentCommit(ci *gc.ChangeInfo) string {
	commit, ok := currentCommit(ci)
	if !ok {
		return ""
	}
	parents := commit.GetParents()
	if len(parents) == 0 {
		return ""
	}
	return parents[0].GetCommit()
}

func flagChips(ci *gc.ChangeInfo) []string {
	var f []string
	if ci.GetWorkInProgress() {
		f = append(f, chip(" WIP ", white, wipBrown))
	}
	if ci.GetIsPrivate() {
		f = append(f, chip(" Private ", white, purple500))
	}
	if ci.GetMergeable() {
		f = append(f, fg("mergeable", green700))
	}
	if ci.GetSubmittable() {
		f = append(f, fg("submittable", green700))
	}
	return f
}

func commentsSummary(ci *gc.ChangeInfo) string {
	total := ci.GetTotalCommentCount()
	unresolved := ci.GetUnresolvedCommentCount()
	resolved := total - unresolved
	if resolved < 0 {
		resolved = 0
	}
	// Match the UI's read: resolved in green, open (unresolved) in red when > 0.
	openColor := green700
	if unresolved > 0 {
		openColor = red600
	}
	return fmt.Sprintf("%d total  (%s, %s)", total,
		fg(fmt.Sprintf("%d resolved", resolved), green700),
		fg(fmt.Sprintf("%d unresolved", unresolved), openColor))
}

func account(a gc.AccountInfo) string {
	name, email := a.GetName(), a.GetEmail()
	switch {
	case name != "" && email != "":
		return named(name, email)
	case name != "":
		return sgr(name, bold)
	case a.GetAccountId() != 0:
		return fmt.Sprintf("account #%d", a.GetAccountId())
	default:
		return "—"
	}
}

func person(p gc.GitPerson) string {
	if p.GetName() == "" && p.GetEmail() == "" {
		return "—"
	}
	return named(p.GetName(), p.GetEmail())
}

// named renders an account/person: bold name, dim <email>. No blue -- reserve blue
// for links.
func named(name, email string) string {
	return fmt.Sprintf("%s %s", sgr(name, bold), sgr("<"+email+">", dim))
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// ---- section / row helpers ----------------------------------------------------

func section(title string) {
	fmt.Println()
	fmt.Printf("  %s\n", sgr(strings.ToUpper(title), bold))
}

func row(label, value string) {
	// Pad the visible label to width BEFORE coloring -- ANSI escape bytes would
	// otherwise be counted by %-14s and break column alignment when color is on.
	fmt.Printf("    %s%s\n", sgr(fmt.Sprintf("%-14s", label), dim), value)
}

// A blue link -- repo/branch, Change-Id, SHAs, URLs.
func link(s string) string { return fg(s, blue700) }

// ---- color / styling ----------------------------------------------------------
// Zero-dependency ANSI, disabled when stdout is not a TTY, on NO_COLOR, or --no-color.

var useColor bool

const (
	bold = "1"
	dim  = "2"
)

type rgb struct{ r, g, b uint8 }

// Borrowed verbatim from Gerrit's Web UI theme (polygerrit-ui app-theme.ts).
var (
	white        = rgb{255, 255, 255}
	black        = rgb{0, 0, 0}
	gray700      = rgb{95, 99, 104}   // --status-merged / --status-abandoned / modified
	yellow700    = rgb{242, 153, 0}   // --status-active (black text)
	wipBrown     = rgb{121, 85, 72}   // --status-wip
	purple500    = rgb{161, 66, 244}  // --status-private / rewrite files
	green700     = rgb{24, 128, 56}   // --status-ready / satisfied / additions
	green300     = rgb{129, 201, 149} // --vote-color-approved chip bg
	red300       = rgb{242, 139, 130} // --vote-color-rejected chip bg
	red600       = rgb{217, 48, 37}   // --status-conflict / deletions
	blue700      = rgb{25, 103, 210}  // links / renamed files
	headerIndigo = rgb{62, 78, 138}   // Gerrit top-bar background, used for rules
)

func computeColor(noColorFlag bool) bool {
	if noColorFlag || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" { // force through a pipe
		return true
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// sgr wraps in an SGR escape (bold/dim) when color is on.
func sgr(s, code string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// fg is a 24-bit foreground (iTerm2 & other truecolor terminals).
func fg(s string, c rgb) string {
	if !useColor {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", c.r, c.g, c.b, s)
}

// chip is a 24-bit filled chip: foreground on background.
func chip(s string, f, b rgb) string {
	if !useColor {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm%s\x1b[0m", f.r, f.g, f.b, b.r, b.g, b.b, s)
}

func rule() string { return fg(strings.Repeat("─", 76), headerIndigo) }

func statusBadge(ci *gc.ChangeInfo) string {
	// Derive the display state the way gr-change-status does: the raw REST status is
	// only NEW/MERGED/ABANDONED, but an open change reads WIP or Private when those
	// flags are set, otherwise "Active".
	label, fgc, bg := "Active", black, yellow700
	switch strings.ToUpper(string(ci.GetStatus())) {
	case "MERGED":
		label, fgc, bg = "Merged", white, gray700
	case "ABANDONED":
		label, fgc, bg = "Abandoned", white, gray700
	default:
		switch {
		case ci.GetWorkInProgress():
			label, fgc, bg = "WIP", white, wipBrown
		case ci.GetIsPrivate():
			label, fgc, bg = "Private", white, purple500
		}
	}
	return chip(" "+label+" ", fgc, bg)
}

func voteChip(v int32, who string) string {
	// Vote chips: Gerrit's approved/rejected backgrounds with dark text.
	bg := green300
	if v < 0 {
		bg = red300
	}
	return fmt.Sprintf("%s %s", chip(fmt.Sprintf(" %+d ", v), black, bg), who)
}

func plusminus(ins, del int32) string {
	return fmt.Sprintf("%s %s", fg(fmt.Sprintf("+%d", ins), green700), fg(fmt.Sprintf("-%d", del), red600))
}

// reqParts returns (icon, colored status text) for a submit-requirement status. The
// wire enum is upper snake case (SATISFIED, NOT_APPLICABLE); display it PascalCased.
func reqParts(status string) (string, string) {
	display := pascal(status)
	switch status {
	case "SATISFIED":
		return fg("✓", green700), fg(display, green700)
	case "UNSATISFIED":
		return fg("✗", red600), fg(display, red600)
	default: // NOT_APPLICABLE and others
		return sgr("○", dim), sgr(display, dim)
	}
}

// pascal turns an upper-snake enum value into PascalCase: NOT_APPLICABLE ->
// NotApplicable, MERGE_IF_NECESSARY -> MergeIfNecessary.
func pascal(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func fileStatus(s string) (string, rgb) {
	switch s {
	case "A":
		return "A", green700 // added
	case "D":
		return "D", red600 // deleted
	case "R":
		return "R", blue700 // renamed
	case "C":
		return "C", blue700 // copied
	case "W":
		return "W", purple500 // rewrite
	default:
		return "M", gray700 // modified
	}
}
