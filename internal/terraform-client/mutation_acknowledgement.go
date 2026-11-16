package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// validateMutationAcknowledgement recognizes the positive response contract of
// each Terraform mutation route. HTTP success alone is not write confirmation.
func validateMutationAcknowledgement(req *http.Request, body []byte) error {
	if req.Method == http.MethodGet {
		return nil
	}
	const prefix = "/api/v1/terraform/"
	index := strings.LastIndex(req.URL.Path, prefix)
	if index < 0 {
		return nil
	}
	parts := strings.Split(strings.Trim(req.URL.Path[index+len(prefix):], "/"), "/")
	if len(parts) < 2 || parts[1] == "create" {
		return nil
	}
	family, action := parts[0], parts[1]
	if action != "update" && action != "delete" && action != "forcedelete" {
		return nil
	}
	fail := func() error { return errors.New("API mutation lacks its route-specific positive acknowledgement") }
	if len(parts) != 3 || parts[2] == "" {
		return fail()
	}
	target := parts[2]
	if family == "authorization" && action == "delete" {
		if len(bytes.TrimSpace(body)) == 0 {
			return nil
		}
		return fail()
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(body, &object)
	stringField := func(key string) string { var v string; _ = json.Unmarshal(object[key], &v); return v }
	trueField := func(key string) bool { return bytes.Equal(bytes.TrimSpace(object[key]), []byte("true")) }
	sameID := func() bool { return validCreateID(stringField("id")) && stringField("id") == target }
	acknowledged := false
	switch family {
	case "resource":
		acknowledged = (action == "update" || action == "delete") && trueField("success")
	case "authorization":
		acknowledged = action == "update" && trueField("ok")
	case "script":
		var message string
		acknowledged = (action == "update" || action == "delete") && json.Unmarshal(body, &message) == nil && message == "success"
	case "team":
		expected := "team updated successfully"
		if action == "delete" {
			expected = "team deleted successfully"
		}
		acknowledged = (action == "update" || action == "delete") && teamMutationAcknowledged(body, expected)
	case "session":
		switch action {
		case "update":
			acknowledged = sameID()
		case "delete":
			acknowledged = sameID() && strings.TrimSpace(stringField("status")) != ""
		case "forcedelete":
			acknowledged = stringField("status") == "session deleted successfully"
		}
	case "schedule":
		if action == "update" {
			acknowledged = sameID() && strings.TrimSpace(stringField("name")) != ""
		}
		if action == "delete" {
			message := stringField("message")
			acknowledged = sameID() && (message == "schedule deleted" || message == "schedule already deleted")
		}
	case "dataprotection":
		if action == "update" {
			acknowledged = sameID() && strings.TrimSpace(stringField("resourceName")) != ""
		}
		if action == "delete" {
			switch stringField("message") {
			case "data protection turned off", "resource is absent, data protection is already off", "no data protection configuration, already off":
				acknowledged = sameID()
			}
		}
	default:
		return nil
	}
	if !acknowledged {
		return fail()
	}
	return nil
}

// Team handlers serialize model.DefaultResponse's untagged Status field. Keep
// the existing lowercase contract too, but require exactly one status field:
// maps and case-insensitive struct decoding would hide duplicate/conflicting
// acknowledgements. Extra fields (including error envelopes) are not this DTO.
func teamMutationAcknowledged(body []byte, expected string) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	key, err := decoder.Token()
	if err != nil || (key != "Status" && key != "status") {
		return false
	}
	var status string
	if decoder.Decode(&status) != nil || status != expected {
		return false
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return false
	}
	_, err = decoder.Token()
	return err == io.EOF
}
