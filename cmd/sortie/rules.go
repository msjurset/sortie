package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/msjurset/sortie/internal/config"
	"github.com/msjurset/sortie/internal/rule"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var rulesFlags struct {
	global bool
}

var rulesCmd = &cobra.Command{
	Use:   "rules [directory...]",
	Short: "List configured rules",
	RunE:  runRules,
}

var rulesTestCmd = &cobra.Command{
	Use:   "test <file>",
	Short: "Show which rule matches a file",
	Args:  cobra.ExactArgs(1),
	RunE:  runRulesTest,
}

func init() {
	rulesCmd.Flags().BoolVar(&rulesFlags.global, "global", false, "include global rules when listing specific directories")
	rulesCmd.AddCommand(rulesTestCmd)
	rootCmd.AddCommand(rulesCmd)
}

func runRules(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		for _, dir := range args {
			dir = expandHome(dir)
			if abs, err := filepath.Abs(dir); err == nil {
				dir = abs
			}

			dc, err := config.LoadDirConfig(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error loading %s: %v\n", dir, err)
				continue
			}

			if dc != nil && len(dc.Rules) > 0 {
				fmt.Printf("%s:\n", dir)
				printRules(dc.Rules)
			} else {
				fmt.Printf("%s: no per-directory rules\n", dir)
			}
		}
		if rulesFlags.global && len(cfg.Rules) > 0 {
			fmt.Println("\nGlobal rules:")
			printRules(cfg.Rules)
		}
		return nil
	}

	if len(cfg.Rules) > 0 {
		fmt.Println("Global rules:")
		printRules(cfg.Rules)
	}

	for _, d := range cfg.Directories {
		dc, err := loadDirRules(d.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error loading %s: %v\n", d.Path, err)
			continue
		}
		if len(dc) > 0 {
			fmt.Printf("\n%s:\n", d.Path)
			printRules(dc)
		}
	}

	if len(cfg.Rules) == 0 {
		hasDirRules := false
		for _, d := range cfg.Directories {
			dc, _ := loadDirRules(d.Path)
			if len(dc) > 0 {
				hasDirRules = true
				break
			}
		}
		if !hasDirRules {
			fmt.Println("No rules configured.")
		}
	}

	return nil
}

func runRulesTest(cmd *cobra.Command, args []string) error {
	path := expandHome(args[0])

	fi, err := rule.NewFileInfo(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Try per-directory rules first, then global
	dir := expandHome(args[0])
	if info, e := os.Stat(dir); e == nil && !info.IsDir() {
		dir = expandHome(args[0])
	}

	rules, err := cfg.MergedRules(fi.Path[:len(fi.Path)-len(fi.Info.Name())-1])
	if err != nil {
		// Fall back to global rules
		rules = cfg.Rules
	}

	mr := rule.FirstMatch(rules, fi)
	if mr == nil {
		fmt.Printf("No rule matches %s\n", fi.Info.Name())
		return nil
	}

	fmt.Printf("File:   %s\n", fi.Info.Name())
	fmt.Printf("Rule:   %s\n", mr.Rule.Name)

	if len(mr.Captures) > 0 {
		for k, v := range mr.Captures {
			fmt.Printf("  Match.%s: %s\n", k, v)
		}
	}

	actions := mr.Rule.ResolvedActions()
	for i, a := range actions {
		prefix := "Action"
		if len(actions) > 1 {
			prefix = fmt.Sprintf("Action[%d]", i+1)
		}
		fmt.Printf("%s: %s\n", prefix, a.Type)
		if a.Dest != "" {
			dest, _ := rule.ExpandTemplate(a.Dest, fi, mr.Captures)
			fmt.Printf("  Dest:   %s\n", dest)
		}
		if extra := summarizeAction(a); extra != "" {
			fmt.Printf("  Detail: %s\n", extra)
		}
	}

	return nil
}

func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return []string{text}
	}
	var lines []string
	for len(text) > maxWidth {
		cut := maxWidth
		if idx := strings.LastIndexByte(text[:maxWidth], ' '); idx > 0 {
			cut = idx
			lines = append(lines, text[:cut])
			text = text[cut+1:]
		} else {
			lines = append(lines, text[:cut])
			text = text[cut:]
		}
	}
	if len(text) > 0 {
		lines = append(lines, text)
	}
	return lines
}

func printRules(rules []rule.Rule) {
	hasPriority := false
	for _, r := range rules {
		if r.Priority != 0 {
			hasPriority = true
			break
		}
	}

	type row struct {
		name, pri, act, match, dest string
	}
	var rows []row
	
	maxName := len("NAME")
	maxPri := len("PRI")
	maxAct := len("ACTION")
	maxMatch := len("MATCH")
	maxDest := len("DEST")

	for _, r := range rules {
		match := summarizeMatch(r.Match)
		actions := r.ResolvedActions()

		var actionStr, destStr string
		if len(actions) <= 1 {
			a := r.Action
			if len(actions) == 1 {
				a = actions[0]
			}
			actionStr = string(a.Type)
			destStr = a.Dest
		} else {
			types := make([]string, len(actions))
			for i, a := range actions {
				types[i] = string(a.Type)
			}
			actionStr = strings.Join(types, " → ")
			destStr = "(chain)"
		}
		
		priStr := ""
		if hasPriority {
			priStr = fmt.Sprintf("%d", r.Priority)
		}

		rows = append(rows, row{r.Name, priStr, actionStr, match, destStr})

		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		if len(priStr) > maxPri {
			maxPri = len(priStr)
		}
		if len(actionStr) > maxAct {
			maxAct = len(actionStr)
		}
		if len(match) > maxMatch {
			maxMatch = len(match)
		}
		if len(destStr) > maxDest {
			maxDest = len(destStr)
		}
	}

	padding := 2
	fixedWidth := 2 + maxName + padding + maxAct + padding
	if hasPriority {
		fixedWidth += maxPri + padding
	}

	termWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth <= 0 {
		termWidth = 120
	}

	allocMatch := maxMatch
	allocDest := maxDest

	if fixedWidth+maxMatch+padding+maxDest > termWidth {
		remaining := termWidth - fixedWidth - padding
		if remaining > 40 {
			// Distribute remaining width between MATCH (60%) and DEST (40%)
			if maxDest < remaining*40/100 {
				allocMatch = remaining - maxDest
			} else if maxMatch < remaining*60/100 {
				allocDest = remaining - maxMatch
			} else {
				allocMatch = remaining * 60 / 100
				allocDest = remaining - allocMatch
			}
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, padding, ' ', 0)
	if hasPriority {
		fmt.Fprintln(w, "  NAME\tPRI\tACTION\tMATCH\tDEST")
	} else {
		fmt.Fprintln(w, "  NAME\tACTION\tMATCH\tDEST")
	}

	for _, row := range rows {
		matchLines := wrapText(row.match, allocMatch)
		destLines := wrapText(row.dest, allocDest)
		
		maxLines := len(matchLines)
		if len(destLines) > maxLines {
			maxLines = len(destLines)
		}
		
		for i := 0; i < maxLines; i++ {
			name := ""
			pri := ""
			act := ""
			match := ""
			dest := ""
			
			if i == 0 {
				name = row.name
				pri = row.pri
				act = row.act
			}
			if i < len(matchLines) {
				match = matchLines[i]
			}
			if i < len(destLines) {
				dest = destLines[i]
			}
			
			if hasPriority {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", name, pri, act, match, dest)
			} else {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", name, act, match, dest)
			}
		}
	}
	w.Flush()
}

func summarizeMatch(m rule.Match) string {
	var parts []string
	if len(m.Extensions) > 0 {
		parts = append(parts, fmt.Sprintf("ext:%v", m.Extensions))
	}
	if m.Glob != "" {
		parts = append(parts, fmt.Sprintf("glob:%s", m.Glob))
	}
	if m.Regex != "" {
		parts = append(parts, fmt.Sprintf("re:%s", m.Regex))
	}
	if m.MinSize != "" {
		parts = append(parts, fmt.Sprintf(">=%s", m.MinSize))
	}
	if m.MaxSize != "" {
		parts = append(parts, fmt.Sprintf("<=%s", m.MaxSize))
	}
	if m.MinAge != "" {
		parts = append(parts, fmt.Sprintf("age>=%s", m.MinAge))
	}
	if m.MaxAge != "" {
		parts = append(parts, fmt.Sprintf("age<=%s", m.MaxAge))
	}
	if m.MimeType != "" {
		parts = append(parts, fmt.Sprintf("mime:%s", m.MimeType))
	}
	if m.Origin != "" {
		parts = append(parts, fmt.Sprintf("origin:%q", m.Origin))
	}
	if m.Content != "" {
		parts = append(parts, fmt.Sprintf("content:%q", m.Content))
	}
	if m.ContentRegex != "" {
		parts = append(parts, fmt.Sprintf("content_re:%s", m.ContentRegex))
	}
	if len(parts) == 0 {
		return "*"
	}
	s := parts[0]
	for _, p := range parts[1:] {
		s += " " + p
	}
	return s
}

func summarizeAction(a rule.Action) string {
	var parts []string

	switch a.Type {
	case rule.ActionChmod:
		parts = append(parts, fmt.Sprintf("mode:%s", a.Mode))
	case rule.ActionChecksum:
		algo := a.Algorithm
		if algo == "" {
			algo = "sha256"
		}
		parts = append(parts, fmt.Sprintf("algorithm:%s", algo))
	case rule.ActionExec:
		parts = append(parts, fmt.Sprintf("command:%s", a.Command))
	case rule.ActionNotify:
		if a.Title != "" {
			parts = append(parts, fmt.Sprintf("title:%s", a.Title))
		}
		if a.Message != "" {
			parts = append(parts, fmt.Sprintf("message:%s", a.Message))
		}
		if a.Link != "" {
			parts = append(parts, fmt.Sprintf("link:%s", a.Link))
		}
	case rule.ActionConvert:
		if a.Tool != "" {
			parts = append(parts, fmt.Sprintf("tool:%s", a.Tool))
		}
		if a.Args != "" {
			parts = append(parts, fmt.Sprintf("args:%s", a.Args))
		}
	case rule.ActionResize:
		if a.Tool != "" {
			parts = append(parts, fmt.Sprintf("tool:%s", a.Tool))
		}
		if a.Width > 0 {
			parts = append(parts, fmt.Sprintf("width:%d", a.Width))
		}
		if a.Height > 0 {
			parts = append(parts, fmt.Sprintf("height:%d", a.Height))
		}
		if a.Percentage > 0 {
			parts = append(parts, fmt.Sprintf("pct:%d%%", a.Percentage))
		}
	case rule.ActionWatermark:
		if a.Tool != "" {
			parts = append(parts, fmt.Sprintf("tool:%s", a.Tool))
		}
		if a.Overlay != "" {
			parts = append(parts, fmt.Sprintf("overlay:%s", a.Overlay))
		}
		if a.Gravity != "" {
			parts = append(parts, fmt.Sprintf("gravity:%s", a.Gravity))
		}
	case rule.ActionOCR:
		if a.Tool != "" {
			parts = append(parts, fmt.Sprintf("tool:%s", a.Tool))
		}
		if a.Language != "" {
			parts = append(parts, fmt.Sprintf("lang:%s", a.Language))
		}
	case rule.ActionEncrypt:
		if a.Tool != "" {
			parts = append(parts, fmt.Sprintf("tool:%s", a.Tool))
		}
		if a.Recipient != "" {
			parts = append(parts, fmt.Sprintf("recipient:%s", a.Recipient))
		}
	case rule.ActionDecrypt:
		if a.Tool != "" {
			parts = append(parts, fmt.Sprintf("tool:%s", a.Tool))
		}
		if a.Key != "" {
			parts = append(parts, fmt.Sprintf("key:%s", a.Key))
		}
	case rule.ActionUpload:
		parts = append(parts, fmt.Sprintf("remote:%s", a.Remote))
	case rule.ActionTag:
		parts = append(parts, fmt.Sprintf("tags:%v", a.Tags))
	case rule.ActionOpen:
		if a.App != "" {
			parts = append(parts, fmt.Sprintf("app:%s", a.App))
		}
	case rule.ActionDeduplicate:
		if a.OnDuplicate != "" {
			parts = append(parts, fmt.Sprintf("on_duplicate:%s", a.OnDuplicate))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	s := parts[0]
	for _, p := range parts[1:] {
		s += " " + p
	}
	return s
}

func loadDirRules(dir string) ([]rule.Rule, error) {
	dc, err := loadDirConfigSafe(dir)
	if err != nil {
		return nil, err
	}
	if dc == nil {
		return nil, nil
	}
	return dc, nil
}

func loadDirConfigSafe(dir string) ([]rule.Rule, error) {
	all, err := cfg.MergedRules(dir)
	if err != nil {
		return nil, err
	}
	// Return only the per-dir rules (those not in global config)
	globalCount := len(cfg.Rules)
	if len(all) > globalCount {
		return all[:len(all)-globalCount], nil
	}
	return nil, nil
}
