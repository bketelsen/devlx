package lxd

import (
	"os"
	"syscall"

	"github.com/bketelsen/libgo/events"
	"github.com/buger/goterm"
	client "github.com/lxc/lxd/client"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/termios"
	"github.com/pkg/errors"
)

type Container struct {
	Name      string
	Etag      string
	conn      client.ContainerServer
	container *api.Container
}

// GetContainer returns the container with the given `name`.
func GetContainer(conn client.ContainerServer, name string) (*Container, error) {
	container, etag, err := conn.GetContainer(name)
	if err != nil {
		return &Container{}, errors.Wrap(err, "getting container")
	}
	return &Container{
		container: container,
		conn:      conn,
		Name:      name,
		Etag:      etag,
	}, nil
}

// Stop causes a running container to cease running.
// An error is returned if the container is not running,
// or if the container doesn't exist.
func (c *Container) Stop() error {
	events.Publish(NewContainerState(c.Name, Stopping))
	cs := api.ContainerStatePut{
		Action: "stop",
	}
	op, err := c.conn.UpdateContainerState(c.Name, cs, c.Etag)
	if err != nil {
		return errors.Wrap(err, "updating container state")
	}
	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for container stop")
	}
	events.Publish(NewContainerState(c.Name, Stopped))
	return nil
}

// Start causes a stopped container to begin running.
// An error is returned if the container doesn't exist,
// or if the container is already running.
func (c *Container) Start() error {
	events.Publish(NewContainerState(c.Name, Starting))
	cs := api.ContainerStatePut{
		Action: "start",
	}
	op, err := c.conn.UpdateContainerState(c.Name, cs, c.Etag)
	if err != nil {
		return errors.Wrap(err, "starting container")
	}
	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for container start")
	}
	events.Publish(NewContainerState(c.Name, Started))
	return nil
}

// Remove deletes a stopped container.  An error is returned
// if the container is not stopped, or if the container doesn't
// exist.
func (c *Container) Remove() error {
	events.Publish(NewContainerState(c.Name, Removing))
	op, err := c.conn.DeleteContainer(c.Name)
	if err != nil {
		return errors.Wrap(err, "deleting container")
	}
	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for container delete")
	}

	events.Publish(NewContainerState(c.Name, Removed))
	return nil
}

func (c *Container) Exec(command string, interactive bool) error {
	events.Publish(NewExecState(c.Name, command, Starting))
	terminalHeight := goterm.Height()
	terminalWidth := goterm.Width()
	// Setup the exec request
	environ := make(map[string]string)
	environ["TERM"] = os.Getenv("TERM")
	req := api.ContainerExecPost{
		Command:     []string{"/bin/bash", "-c", "sudo --user ubuntu --login" + " " + command},
		WaitForWS:   true,
		Interactive: interactive,
		Width:       terminalWidth,
		Height:      terminalHeight,
		Environment: environ,
	}

	// Setup the exec arguments (fds)
	largs := lxd.ContainerExecArgs{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Setup the terminal (set to raw mode)
	if req.Interactive {
		cfd := int(syscall.Stdin)
		oldttystate, err := termios.MakeRaw(cfd)
		if err != nil {
			return errors.Wrap(err, "error making raw terminal")
		}

		defer termios.Restore(cfd, oldttystate)
	}

	// Get the current state
	op, err := c.conn.ExecContainer(c.Name, req, &largs)
	if err != nil {
		errors.Wrap(err, "execution error")
	}

	events.Publish(NewExecState(c.Name, command, Started))
	// Wait for it to complete
	err = op.Wait()
	if err != nil {
		errors.Wrap(err, "error waiting for execution")
	}

	events.Publish(NewExecState(c.Name, command, Completed))
	return nil
}
