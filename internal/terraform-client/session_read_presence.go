package client

import (
	"encoding/json"
	"strings"
)

// UnmarshalJSON retains presence without changing public scalar field types.
// Missing/null scalar policies are unknown, not authoritative false or zero.
func (r *SessionReadResponse) UnmarshalJSON(data []byte) error {
	type plain SessionReadResponse
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = SessionReadResponse(decoded)
	r.policyFields = make(map[string]bool, len(fields))
	for key, value := range fields {
		key = strings.ToLower(key)
		// The backend serializes a nil approver slice as null; this is an
		// authoritative empty list, unlike a missing list or null boolean.
		r.policyFields[key] = key == "accessapprovers" || strings.TrimSpace(string(value)) != "null"
	}
	return nil
}

// HasField reports whether JSON supplied an authoritative value. For scalar
// fields null is unknown; a present null approver list represents an empty list.
// Metadata is private and is never serialized into requests or Terraform state.
func (r *SessionReadResponse) HasField(key string) bool {
	return r != nil && r.policyFields[strings.ToLower(key)]
}
