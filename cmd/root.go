package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "repited",
	Short: "repited is a CLI application",
	Long: `repited is a CLI application

This is a CLI application built with Cobra.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Version = GetVersionJSON()
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
}
