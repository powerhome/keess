/*
Copyright © 2024 Marcus Vinicius <mvleandro@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "keess",
	Short: "Synchronize secrets and configmaps across namespaces and clusters.",
	// Long: `Keess (Keep Stuff Synchronized) is a command-line tool designed to operate within Kubernetes clusters, focusing on the synchronization of secrets and configmaps. It efficiently handles the transfer of these critical components across different namespaces within a single cluster, as well as across multiple clusters, ensuring consistency and security in distributed environments.

	// Key Features:
	// - Cross-Namespace Synchronization: Sync secrets and configmaps across various namespaces in a single cluster.
	// - Inter-Cluster Synchronization: Extend synchronization capabilities to multiple Kubernetes clusters.
	// - Security-Oriented: Ensures secure transfer, maintaining data integrity and confidentiality.
	// - Automation and Efficiency: Automates synchronization processes, reducing manual effort and error.
	// - Customizable Rules: Offers flexibility in defining specific synchronization conditions.
	// - User-Friendly CLI: Easy-to-use command-line interface for seamless integration and operation.
	// - Logging and Monitoring: Provides comprehensive logs for tracking and auditing purposes.

	// Keess streamlines Kubernetes configuration management, making it an essential tool for DevOps teams and Kubernetes administrators seeking to maintain uniformity and security across their infrastructure.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.keess.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".keess" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".keess")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
