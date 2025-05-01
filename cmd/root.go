package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set by main
	version string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "envctl",
	Short: "Connect your environment to Giant Swarm clusters",
	Long: `envctl simplifies connecting your local development environment
(e.g., MCP servers in Cursor) to Giant Swarm clusters via Teleport
and setting up necessary connections like Prometheus port-forwarding.`,
	// SilenceUsage is set to true to prevent printing usage message on errors
	// handled by us (e.g. invalid arguments, failed connections)
	SilenceUsage: true,
}

// SetVersion sets the version for the root command
func SetVersion(v string) {
	version = v
	rootCmd.Version = v // Set cobra's version field as well
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Set up version template
	rootCmd.SetVersionTemplate(`{{printf "envctl version %s\n" .Version}}`)

	err := rootCmd.Execute()
	if err != nil {
		// Cobra prints the error, we just exit non-zero
		os.Exit(1)
	}
}

func init() {
	// Add subcommands here, e.g.:
	// rootCmd.AddCommand(newCompletionCmd())
	rootCmd.AddCommand(newConnectCmd())
	rootCmd.AddCommand(newVersionCmd()) // Add version command
	rootCmd.AddCommand(newSelfUpdateCmd()) // Add self-update command

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.envctl.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
