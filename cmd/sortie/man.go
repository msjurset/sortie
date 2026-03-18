package main

import (
	"fmt"

	"github.com/msjurset/sortie/internal/manpage"
	"github.com/spf13/cobra"
)

var manCmd = &cobra.Command{
	Use:   "man",
	Short: "Display the sortie manual page",
	Long:  "Print the roff-formatted man page to stdout. Use with: sortie man | man -l -",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(manpage.Content)
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
