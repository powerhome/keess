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

	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Initiates the synchronization of secrets and configmaps.",
	// Long: `The 'run' command in Keess is the primary trigger for initiating the synchronization process of secrets and configmaps across Kubernetes namespaces and clusters. When executed, it activates Keess’s core functionality, seamlessly transferring specified configurations and sensitive data according to predefined rules and parameters. This command ensures that all targeted Kubernetes environments are updated with the latest configurations and secrets, maintaining consistency and enhancing security across your distributed infrastructure.

	// Features:
	// - Automated Synchronization: Executes the automated process of syncing secrets and configmaps.
	// - Cross-Namespace and Cluster Operation: Works across different namespaces and multiple clusters.
	// - Secure Transfer: Adheres to strict security protocols to ensure safe data transfer.
	// - Custom Synchronization: Respects user-defined rules and conditions for targeted synchronization.
	// - Real-Time Execution: Performs synchronization in real time, ensuring timely updates.
	// - Logging: Generates logs for the synchronization process, aiding in monitoring and auditing.

	// Usage of the 'run' command is essential for keeping your Kubernetes environments synchronized and secure, forming the backbone of Keess's operational capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run called")
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
