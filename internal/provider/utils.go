package provider

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	ErrResourceNameNotFound = errors.New("resource name not found")
	ErrAttributeNotFound    = errors.New("attribute not found")
	ErrAttributeBadType     = errors.New("attribute has bad type")
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

func safeAttributeFromSchema(d *schema.ResourceData, key string) (value interface{}, err error) {
	_value, ok := d.GetOk(key)
	if !ok {
		return nil, nil
	}
	return _value, nil
}

func attrFromSchema[T any](d *schema.ResourceData, attr string, required bool) (attrValue *T, err error) {
	_name, ok := d.GetOk(attr)
	if required && !ok {
		return nil, fmt.Errorf("attribute: %s, not found", attr)
		// return nil, ErrAttributeNotFound
	}
	if value, okk := _name.(T); required && !okk {
		// return nil, ErrAttributeBadType
		return nil, fmt.Errorf("attribute: %s, expected type: %T", attr, attrValue)
	} else {
		return &value, nil
	}
}

func tagsFromSchema(d *schema.ResourceData) ([]string, error) {
	var userTags []string
	tags := d.Get("tags")
	if tags != nil {
		// Convert the tags to a slice of strings
		if val, ok := tags.([]interface{}); !ok {
			return nil, fmt.Errorf("tags must be a list of strings, got %T", tags)
		} else {
			for _, tag := range val {
				userTags = append(userTags, tag.(string))
			}
		}
	}

	return userTags, nil
}
