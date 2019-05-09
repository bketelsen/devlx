// Copyright (c) 2019 bketelsen
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"devlx/lxd"
	"devlx/path"

	"github.com/lxc/lxd/shared/i18n"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// represents an LXD Image
type Image struct {
	Fingerprint string `yaml:"fingerprint,omitempty"`
}

// represents a template
type Template struct {
	Name   string `yaml:"name"`
	UsedBy string `yaml:"usedBy,omitempty"`
	Image  *Image `yaml:"images,omitempty"`
}

// represents the template "collection"
type Templates struct {
	Templates []Template `yaml:"templates"`
}

// store the container template (image) relation in yaml file
// takes as arguments the container name, relating template name
// for store set variable store true
func setContainerTemplateRelation(c *lxd.Client, container string, tmpl string, store bool) error {
	var templates Templates

	// store the relation
	if store {

		err := templates.parse()
		if err != nil {
			return errors.Wrap(err, "Failed parse data")
		}

		// append template users which is a container
		// if an entry is already there, then return to caller
		for i, template := range templates.Templates {
			if tmpl == template.Name {

				sep := strings.Split(template.UsedBy, ",")
				for _, usedby := range sep {
					if usedby == container {
						return fmt.Errorf(i18n.G("Entry already here nothing to do"))
					}
				}

				templates.Templates[i].UsedBy += container + ","
				err = templates.store()
				if err != nil {
					return errors.Wrap(err, "Failed storing")
				}
				return nil
			}
		}

		fingerprint, err := c.GetImageFingerprint(tmpl)
		if err != nil {
			return errors.Wrap(err, "Failed getting fingerprint")
		}

		// if not create a new entry
		err = writeEntry(&templates, container, tmpl, fingerprint)
		if err != nil {
			return errors.Wrap(err, "Failed storing")
		}

	} else {
		// else delete the relation

		err := templates.parse()
		if err != nil {
			return errors.Wrap(err, "Failed parse yaml data")
		}

		if templates.Templates == nil {
			return fmt.Errorf(i18n.G("Error no LXC Image entry here, maybe something went wrong?"))
		}

		for i, template := range templates.Templates {
			sep := strings.Split(template.UsedBy, ",")
			for j, str := range sep {
				if container == str {
					copy(sep[j:], sep[j+1:])
					sep[len(sep)-1] = ""
					sep = sep[:len(sep)-1]
					templates.Templates[i].UsedBy = strings.Join(sep, ",")
					err = templates.store()
					if err != nil {
						return err
					}
					break
				}
			}
		}
	}

	return nil
}

// write container - template (image) relation
func writeEntry(t *Templates, container string, tmpl string, fingerprint string) error {
	img := &Image{Fingerprint: fingerprint}

	if container == "" {
		fmt.Println("Storing: ", tmpl, "in yaml.")
		template := Template{Name: tmpl, Image: img}
		t.Templates = append(t.Templates, template)
	} else {
		template := Template{Name: tmpl, UsedBy: container, Image: img}
		t.Templates = append(t.Templates, template)
	}

	err := t.store()
	if err != nil {
		return err
	}

	return nil
}

// Removing the templates which is an LXC image
func removeTemplate(c *lxd.Client, tmpl string) error {
	var templates Templates
	var success bool = false

	fingerprint, err := c.GetImageFingerprint(tmpl)
	if err != nil {
		return errors.Wrap(err, "Failed getting LCX Image fingerprint")
	} else if fingerprint == "" {
		return fmt.Errorf(i18n.G("Error there is no " + tmpl + " (anymore) here. Giving up nothing to remove."))
	}

	err = templates.parse()
	if err != nil {
		return errors.Wrap(err, "Failed parse yaml data")
	}

	for i, template := range templates.Templates {
		if template.UsedBy == "" && template.Name == tmpl {
			err = c.ContainerRemove(tmpl)
			if err != nil {
				cause := err.Error()
				switch cause {
				case "not found":
					log.Error("Error container " + tmpl + " not found, maybe it's deleted accidentally, I try to remove the related LXC Image.....")
					suberr := lxd.RemoveTemplateImage(c, fingerprint)
					if suberr != nil {
						return errors.Wrap(err, "Failed remove LXC Image "+tmpl)
					}
					log.Success("I was able to remove the template LXC Image")
					success = true
				default:
					return errors.Wrap(err, "Failed remove template container")
				}
			}
			if !success {
				err = lxd.RemoveTemplateImage(c, fingerprint)
				if err != nil {
					return errors.Wrap(err, "Failed remove template LXC Image "+tmpl)
				}
			}
			copy(templates.Templates[i:], templates.Templates[i+1:])
			templates.Templates[len(templates.Templates)-1] = Template{}
			templates.Templates = templates.Templates[:len(templates.Templates)-1]
			err = templates.store()
			if err != nil {
				return errors.Wrap(err, "Failed parse yaml data")
			}
			break
		} else if template.Name == tmpl {
			return fmt.Errorf(i18n.G("Error can not remove LXC Image, it's still in use by " + strings.Trim(template.UsedBy, ",") + " !"))
		}
	}

	return nil
}

// Unmarshal helper function
// path is hardcoded?
func (t *Templates) parse() error {
	filename := filepath.Join(path.GetConfigPath(), "templates", "relations.yaml")

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, t); err != nil {
		return err
	}

	return nil
}

// Marshal helper function
func (t *Templates) store() error {
	filename := filepath.Join(path.GetConfigPath(), "templates", "relations.yaml")

	bytes, err := yaml.Marshal(t)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, bytes, 0644)
}
