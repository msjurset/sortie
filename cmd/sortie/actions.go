package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/msjurset/sortie/internal/actionhelp"
	"github.com/spf13/cobra"
)

var actionsCmd = &cobra.Command{
	Use:   "actions [name]",
	Short: "List action types or show detailed help for a specific action",
	Long: `List all available action types, or show detailed help including
required fields, optional fields, a YAML config example, tips, and
practical use cases for a specific action.

Examples:
  sortie actions            # list all action types
  sortie actions convert    # show detailed help for the convert action
  sortie help convert       # same thing (also works via cobra help)`,
	RunE: runActions,
}

func init() {
	rootCmd.AddCommand(actionsCmd)

	// Register hidden commands for each action type so that
	// "sortie help exec", "sortie help convert", etc. work via cobra's
	// built-in help routing.
	for _, h := range actionhelp.List() {
		help := h // capture loop variable
		cmd := &cobra.Command{
			Use:    help.Name,
			Short:  help.Description,
			Hidden: true,
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println(actionhelp.Format(help))
			},
		}
		// Override the help function so "sortie help exec" shows our
		// rich format instead of cobra's default template.
		cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
			fmt.Println(actionhelp.Format(help))
		})
		rootCmd.AddCommand(cmd)
	}

	// Add action type completions to the built-in "help" command so that
	// "sortie help e<TAB>" completes to "exec", "encrypt", etc.
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "help" {
			cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				if len(args) > 0 {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				// Combine subcommand names + action type names
				var completions []string
				for _, sub := range rootCmd.Commands() {
					if !sub.Hidden && sub.Name() != "help" {
						completions = append(completions, sub.Name()+"\t"+sub.Short)
					}
				}
				for _, h := range actionhelp.List() {
					completions = append(completions, h.Name+"\tAction: "+h.Name)
				}
				return completions, cobra.ShellCompDirectiveNoFileComp
			}
			break
		}
	}
}

func runActions(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		h, ok := actionhelp.Get(args[0])
		if !ok {
			return fmt.Errorf("unknown action type %q — run 'sortie actions' to see all types", args[0])
		}
		fmt.Println(actionhelp.Format(h))
		return nil
	}

	// List all action types
	list := actionhelp.List()
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ACTION\tDESCRIPTION\tUNDOABLE")
	for _, h := range list {
		undo := "No"
		if h.Undoable {
			undo = "Yes"
		}
		// Truncate description to ~55 chars for table display
		desc := h.Description
		if len(desc) > 55 {
			desc = desc[:52] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", h.Name, desc, undo)
	}
	w.Flush()

	fmt.Printf("\nRun 'sortie actions <name>' for detailed help on a specific action.\n")
	return nil
}
