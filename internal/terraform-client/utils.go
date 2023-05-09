package client

import (
	"github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client/adaptive"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func interfaceToStr(val interface{}) string {
	if val, ok := val.(string); !ok {
		return ""
	} else {
		return val
	}
}

func schemaMapToResource(d *schema.ResourceData) adaptive.Resource {
	if list := d.Get("aks").([]interface{}); len(list) > 0 {
		raw, ok := list[0].(map[string]interface{})
		if !ok {
			return &adaptive.Mongo{}
		}
		out := &adaptive.Mongo{
			ID:   d.Id(),
			Name: interfaceToStr(raw["name"]),
			Uri:  interfaceToStr(raw["uri"]),
		}
		return out
	}
	return nil
}
