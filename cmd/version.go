package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version of the application, set this variable during build
var Version = "1.4.1"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",

	Short: "Print the version number of the application",
	Long:  `Print the version number of the application`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Keess v" + Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
