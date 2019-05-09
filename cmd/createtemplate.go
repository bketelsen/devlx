// Copyright (c) 2019 bketelsen
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package cmd

import (
	"os"
	"time"

	client "devlx/lxd"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var provisioners *[]string
var base string
var guiimage string
var cliimage string

// createtemplateCmd represents the createtemplate command
var createtemplateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a template",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name = args[0]
		log.Running("Creating template " + name)
		// Connect to LXD over the Unix socket
		// TODO: account for non snap install

		lxclient, err := client.NewClient(socket)
		if err != nil {
			log.Error("Unable to connect: " + err.Error())
			os.Exit(1)
		}

		var kind client.Type
		if base == "cli" {
			kind = client.CLI
		} else {
			kind = client.GUI
		}
		// create the container
		err = lxclient.ContainerCreate(name, false, getTemplateImage(kind), getProfiles())
		if err != nil {
			log.Error("Unable to create template: " + err.Error())
			os.Exit(1)
		}

		log.Running("Container starting: " + name) // need better plan here
		time.Sleep(10 * time.Second)
		err = lxclient.ContainerProvision(name, kind, *provisioners)
		if err != nil {
			log.Error("Provisioning template: " + err.Error())
			os.Exit(1)
		}

		// snapshot the container
		err = lxclient.ContainerSnapshot(name, "template")

		if err != nil {
			log.Error("Creating snapshot: " + err.Error())
			os.Exit(1)
		}

		// publish the container
		err = lxclient.ContainerPublish(name)
		if err != nil {
			log.Error("Publishing image: " + err.Error())
			os.Exit(1)
		}

		err = lxclient.ContainerStop(name)
		if err != nil {
			log.Error("Error stopping container: " + err.Error())
			os.Exit(1)
		}

		// evaluate template in relations.yaml
		err = setContainerTemplateRelation(lxclient, "", name, true)
		if err != nil {
			log.Error("Error writing template to database: " + err.Error())
			os.Exit(1)
		}
	},
}

func getTemplateImage(kind client.Type) string {
	if kind == client.CLI {
		return viper.GetString("cliimage")
	}
	return viper.GetString("guiimage")
}
func init() {
	templateCmd.AddCommand(createtemplateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	templateCmd.PersistentFlags().StringVar(&base, "profile", "gui", "Base profile (gui or cli)")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createtemplateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	templateCmd.PersistentFlags().StringVar(&guiimage, "guiimage", "18.10", "Ubuntu version for GUI instances")
	viper.BindPFlag("guiimage", templateCmd.PersistentFlags().Lookup("guiimage"))

	templateCmd.PersistentFlags().StringVar(&cliimage, "cliimage", "18.10", "Ubuntu version for CLI instances")
	viper.BindPFlag("cliimage", templateCmd.PersistentFlags().Lookup("cliimage"))
	provisioners = templateCmd.PersistentFlags().StringSlice("provisioners", []string{}, "Comma separated list of provision scripts to run . e.g. 'go,neovim'")
}
