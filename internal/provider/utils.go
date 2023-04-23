package provider

import (
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	ErrResourceNameNotFound = errors.New("resource name not found")
)

func nameFromSchema(d *schema.ResourceData) (name string, err error) {
	_name, ok := d.GetOk("name")
	if !ok {
		return "", ErrResourceNameNotFound
	}
	if name, okk := _name.(string); !okk || name == "" {
		return "", ErrResourceNameNotFound
	} else {
		return name, nil
	}
}
