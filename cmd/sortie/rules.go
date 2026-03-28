package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/msjurset/sortie/internal/rule"
	"github.com/spf13/cobra"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "List configured rules",
	Args:  cobra.NoArgs,
	RunE:  runRules,
}

var rulesTestCmd = &cobra.Command{
	Use:   "test <file>",
	Short: "Show which rule matches a file",
	Args:  cobra.ExactArgs(1),
	RunE:  runRulesTest,
}

func init() {
	rulesCmd.AddCommand(rulesTestCmd)
	rootCmd.AddCommand(rulesCmd)
}

func runRules(cmd *cobra.Command, args []string) error {
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

	matched := rule.FirstMatch(rules, fi)
	if matched == nil {
		fmt.Printf("No rule matches %s\n", fi.Info.Name())
		return nil
	}

	dest, _ := rule.ExpandTemplate(matched.Action.Dest, fi)
	fmt.Printf("File:   %s\n", fi.Info.Name())
	fmt.Printf("Rule:   %s\n", matched.Name)
	fmt.Printf("Action: %s\n", matched.Action.Type)
	if dest != "" {
		fmt.Printf("Dest:   %s\n", dest)
	}
	if extra := summarizeAction(matched.Action); extra != "" {
		fmt.Printf("Detail: %s\n", extra)
	}

	return nil
}

func printRules(rules []rule.Rule) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tACTION\tMATCH\tDEST")
	for _, r := range rules {
		match := summarizeMatch(r.Match)
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", r.Name, r.Action.Type, match, r.Action.Dest)
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
