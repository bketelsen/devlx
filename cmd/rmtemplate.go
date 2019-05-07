// Copyright (c) 2019 bketelsen
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package cmd

import (
	"os"

	client "devlx/lxd"

	"github.com/spf13/cobra"
)

// rmCmd represents the rm command
var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove a template",
	Long:  `Remove a previously configured template.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		template := args[0]

		//Connect to LXD over the Unix Socket
		lxclient, err := client.NewClient(config.lxdSocket)
		if err != nil {
			log.Error("Unable to connect: " + err.Error())
			os.Exit(1)
		}

		// Remove the template which is an LXC image
		log.Running("Try to remove template: " + template)
		err = removeTemplate(lxclient, template)
		if err != nil {
			log.Error("Unable to remove template: " + err.Error())
			os.Exit(1)
		}
		log.Success("Template removed")
	},
}

func init() {
	templateCmd.AddCommand(rmCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// rmCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// rmCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
