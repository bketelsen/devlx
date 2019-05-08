// Copyright © 2019 bketelsen
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package cmd

import (
	"os"
	"strings"

	client "devlx/lxd"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:     "exec [container] '[commands here]'",
	Aliases: []string{"run"},
	Short:   "Execute a command in a container",
	Long: `Executes a command in the named container.  The command should be enclosed in 
single quotes.  e.g. exec mycontainer 'ls -la'`,
	Run: func(cmd *cobra.Command, args []string) {
		name = args[0]
		// Connect to LXD over the Unix socket
		lxclient, err := client.NewClient(config.LxdSocket)
		if err != nil {
			log.Error("Unable to connect: " + err.Error())
			os.Exit(1)
		}
		err = lxclient.ContainerExec(name, strings.Join(args[1:], " "))
		if err != nil {
			log.Error("Error executing command: " + err.Error())
			os.Exit(1)
		}

	},
}

func init() {
	rootCmd.AddCommand(execCmd)
}
