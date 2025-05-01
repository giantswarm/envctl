package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of envctl",
		Long:  `All software has versions. This is envctl's.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Use the rootCmd.Version which is set in root.go
			// The version template in root.go handles the actual printing
			// So this Run function can be minimal or even omitted if
			// the root command itself handles the -v/--version flag.
			// However, having an explicit command is standard.
			fmt.Printf("envctl version %s\n", rootCmd.Version)
		},
	}
} 