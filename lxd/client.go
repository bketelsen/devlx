// Copyright (c) 2019 bketelsen
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package lxd

import (
	"fmt"
	"github.com/bketelsen/libgo/events"
	"github.com/lxc/lxd/client"
	client "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/i18n"
	"github.com/pkg/errors"
	"sort"
)

type Client struct {
	URL string

	conn client.ContainerServer
}

// NewClient creates a new connection to an LXD Daemon,
// returning a Client
func NewClient(url string) (*Client, error) {
	c := &Client{
		URL: url,
	}
	err := c.Connect()
	return c, err
}

// Connect establishes a connection to an LXD Daemon
func (c *Client) Connect() error {
	var err error
	c.conn, err = client.ConnectLXDUnix(c.URL, nil)
	if err != nil {
		return errors.Wrap(err, "Error connecting to LXD daemon")
	}

	events.Publish(NewConnectionCreated(c))
	return nil
}

func (c *Client) ContainerProvision(name string, kind Type, provisioners []string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return errors.Wrap(err, "getting container")
	}
	return cont.Provision(kind, provisioners)
}

func (c *Client) ContainerShell(name string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return errors.Wrap(err, "getting container")
	}
	return cont.Exec("", true)
}

func (c *Client) ContainerCreate(name string, isAlias bool, image string, profiles []string) error {
	// Container creation request
	var source api.ContainerSource
	if isAlias {
		source = api.ContainerSource{
			Type: "image",
			//Server:   "https://cloud-images.ubuntu.com/daily",
			//Alias:    getImage(),
			Alias: image,
			//Protocol: "simplestreams",
		}
	} else {
		source = api.ContainerSource{
			Type:     "image",
			Server:   "https://cloud-images.ubuntu.com/daily",
			Alias:    image,
			Protocol: "simplestreams",
		}
	}
	req := api.ContainersPost{
		Name: name,
		ContainerPut: api.ContainerPut{
			Profiles: profiles,
		},
		Source: source,
	}

	events.Publish(NewContainerState(name, Creating))

	// Get LXD to create the container (background operation)
	op, err := c.conn.CreateContainer(req)
	if err != nil {
		return errors.Wrap(err, "creating container")
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "wait for create container")
	}

	events.Publish(NewContainerState(name, Created))
	// Get LXD to start the container (background operation)
	reqState := api.ContainerStatePut{
		Action:  "start",
		Timeout: -1,
	}

	events.Publish(NewContainerState(name, Starting))
	op, err = c.conn.UpdateContainerState(name, reqState, "")
	if err != nil {
		return errors.Wrap(err, "starting container")
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for container start")
	}

	events.Publish(NewContainerState(name, Started))
	return nil
}
func (c *Client) ContainerExec(name string, command string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return errors.Wrap(err, "getting container")
	}
	return cont.Exec(command, false)
}

func (c *Client) ContainerList() ([]string, error) {
	names, err := c.conn.GetContainerNames()
	if err != nil {
		errors.Wrap(err, "get container names")
	}
	return names, err
}

func (c *Client) ContainerInfo(name string) (*api.Container, error) {
	container, _, err := c.conn.GetContainer(name)
	if err != nil {
		errors.Wrap(err, "get container names")
	}
	return container, err
}

func (c *Client) ContainerRemove(name string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return err
	}
	return cont.Remove()
}

func (c *Client) ContainerStart(name string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return errors.Wrap(err, "getting container")
	}
	return cont.Start()
}

func (c *Client) ContainerStop(name string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return errors.Wrap(err, "getting container")
	}
	return cont.Stop()
}

func (c *Client) ImageList() ([]api.Image, error) {
	return c.conn.GetImages()

}
func (c *Client) ContainerSnapshot(name string, snapshotName string) error {
	cont, err := GetContainer(c.conn, name)
	if err != nil {
		return errors.Wrap(err, "getting container")
	}
	return cont.Snapshot(snapshotName)
}

func (c *Client) ContainerPublish(name string) error {

	fmt.Println("Publishing: ", name)
	// Create the image
	req := api.ImagesPost{
		Source: &api.ImagesPostSource{
			Type: "container",
			Name: name + "/template", // UGH?

		},
	}
	// skipping properties, that may be a mistake?
	req.Source.Type = "snapshot"
	req.Public = true

	alias := api.ImageAlias{}
	alias.Name = name
	alias.Description = "devlx template: " + name
	op, err := c.conn.CreateImage(req, nil)
	if err != nil {
		return errors.Wrap(err, "create container image")
	}
	// Wait for it to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "wait for operation")
	}
	opAPI := op.Get()

	// Grab the fingerprint
	fingerprint := opAPI.Metadata["fingerprint"].(string)
	return ensureImageAliases(c.conn, []api.ImageAlias{alias}, fingerprint)

}

// Create the specified image alises, updating those that already exist
// copied from lxd source :)
func ensureImageAliases(client lxd.ContainerServer, aliases []api.ImageAlias, fingerprint string) error {
	if len(aliases) == 0 {
		return nil
	}

	names := make([]string, len(aliases))
	for i, alias := range aliases {
		names[i] = alias.Name
	}
	sort.Strings(names)

	resp, err := client.GetImageAliases()
	if err != nil {
		return err
	}

	// Delete existing aliases that match provided ones
	for _, alias := range GetExistingAliases(names, resp) {
		err := client.DeleteImageAlias(alias.Name)
		if err != nil {
			fmt.Println(fmt.Sprintf(i18n.G("Failed to remove alias %s"), alias.Name))
		}
	}
	// Create new aliases
	for _, alias := range aliases {
		aliasPost := api.ImageAliasesPost{}
		aliasPost.Name = alias.Name
		aliasPost.Target = fingerprint
		err := client.CreateImageAlias(aliasPost)
		if err != nil {
			fmt.Println(fmt.Sprintf(i18n.G("Failed to create alias %s"), alias.Name))
		}
	}
	return nil
}

// GetExistingAliases returns the intersection between a list of aliases and all the existing ones.
func GetExistingAliases(aliases []string, allAliases []api.ImageAliasesEntry) []api.ImageAliasesEntry {
	existing := []api.ImageAliasesEntry{}
	for _, alias := range allAliases {
		name := alias.Name
		pos := sort.SearchStrings(aliases, name)
		if pos < len(aliases) && aliases[pos] == name {
			existing = append(existing, alias)
		}
	}
	return existing
}

// retrieves the image fingerprint from image name
func (c *Client) GetImageFingerprint(image string) (string, error) {
	var retVal string
	imagesAPI, err := c.conn.GetImages()

	if err != nil {
		return "", fmt.Errorf(i18n.G("Failed getting container metadata"))
	}

	for _, imageAPI := range imagesAPI {
		for _, alias := range imageAPI.Aliases {
			if alias.Name == image {
				retVal = imageAPI.Fingerprint
			}
		}
	}
	return retVal, nil
}

// helper function deletes the LXC image
func RemoveTemplateImage(c *Client, fingerprint string) error {
	op, err := c.conn.DeleteImage(fingerprint)
	if err != nil {
		return errors.Wrap(err, "Failed to remove image")
	}

	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for removing image")
	}
	return nil
}
