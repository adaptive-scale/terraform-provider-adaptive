package client

import "encoding/json"

// UnmarshalJSON selects aliases by presence, never by a scalar's zero value.
// An explicitly empty canonical field remains authoritative.
func (r *AuthorizationReadResponse) UnmarshalJSON(data []byte) error {
	type wire AuthorizationReadResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for canonical, legacy := range map[string]string{"permissions": "legacyPermission", "resourceType": "resource_type"} {
		if _, exists := fields[canonical]; exists {
			continue
		}
		raw, exists := fields[legacy]
		if !exists {
			continue
		}
		target := &decoded.Permissions
		if canonical == "resourceType" {
			target = &decoded.ResourceType
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return err
		}
	}
	*r = AuthorizationReadResponse(decoded)
	_, canonical := fields["resourceType"]
	_, legacy := fields["resource_type"]
	r.ResourceTypePresent = canonical || legacy
	return nil
}
