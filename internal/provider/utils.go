package provider

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	ErrResourceNameNotFound = errors.New("resource name not found")
	ErrAttributeNotFound    = errors.New("attribute not found")
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

func attrFromSchema[T any](d *schema.ResourceData, attr string, required bool) (attrValue *T, err error) {
	_name, ok := d.GetOk(attr)
	if required && !ok {
		return nil, fmt.Errorf("get %s %w", attr, ErrAttributeNotFound)
	}
	if value, okk := _name.(T); required && !okk {
		return nil, fmt.Errorf("%s %w", attr, ErrAttributeNotFound)
	} else {
		return &value, nil
	}
}
