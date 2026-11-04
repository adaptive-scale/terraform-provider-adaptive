package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

const maxResponseBytes = 4 << 20

// HTTPError retains the status without exposing arbitrary server messages, which
// can contain submitted credentials/configuration. Unresolved names are a
// dedicated backend DTO, not a free-form error string.
type HTTPError struct {
	Reason                                                string
	StatusCode                                            int
	UnresolvedUsers, UnresolvedTeams, UnresolvedEndpoints []string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("Adaptive API status %d (%s)", e.StatusCode, http.StatusText(e.StatusCode))
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.StatusCode == 401 {
		msg += ": bad token; check the service token"
	}
	for _, v := range []struct {
		label string
		names []string
	}{{"users", e.UnresolvedUsers}, {"teams", e.UnresolvedTeams}, {"endpoints", e.UnresolvedEndpoints}} {
		if len(v.names) > 0 {
			msg += fmt.Sprintf("; unresolved %s %q", v.label, v.names)
		}
	}
	return msg
}
func newHTTPError(status int, body []byte, req *http.Request) *HTTPError {
	e := &HTTPError{StatusCode: status}
	var decoded any
	if json.Unmarshal(body, &decoded) != nil {
		return e
	}
	message, _ := decoded.(string)
	object, _ := decoded.(map[string]any)
	if message == "" {
		message, _ = object["error"].(string)
		if message == "" {
			message, _ = object["msg"].(string)
		}
	}
	// Exact backend constants only, never arbitrary request/error payloads.
	switch message {
	case "failed to get session", "session is not terminated", "user not allowed", "invalid request", "failed to mark session for deletion", "failed to delete session":
		e.Reason = message
	}
	// Only expose dedicated unresolved names also present in submitted identifier
	// lists. Decode the error body once; never interpret arbitrary text as names.
	if status == 400 && strings.Contains(req.URL.Path, "/schedule/") && req.GetBody != nil {
		if r, err := req.GetBody(); err == nil {
			b, readErr := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
			r.Close()
			var input struct{ Users, Teams, Endpoints []string }
			if readErr == nil && len(b) <= maxResponseBytes && json.Unmarshal(b, &input) == nil {
				names := func(key string) []string {
					var result []string
					values, _ := object[key].([]any)
					for _, value := range values {
						if name, ok := value.(string); ok {
							result = append(result, name)
						}
					}
					return result
				}
				e.UnresolvedUsers = safeUnresolved(names("unresolvedUsers"), input.Users)
				e.UnresolvedTeams = safeUnresolved(names("unresolvedTeams"), input.Teams)
				e.UnresolvedEndpoints = safeUnresolved(names("unresolvedEndpoints"), input.Endpoints)
			}
		}
	}
	return e
}
func safeUnresolved(names, requested []string) []string {
	allowed := make(map[string]bool, len(requested))
	for _, name := range requested {
		allowed[name] = true
	}
	var result []string
	for _, name := range names {
		if len(result) >= 32 {
			break
		}
		if allowed[name] && name != "" && len(name) <= 256 && strings.IndexFunc(name, unicode.IsControl) < 0 {
			result = append(result, name)
			delete(allowed, name)
		}
	}
	return result
}

// decodeResponse deliberately does not wrap decoder errors: type/overflow
// errors can contain response values, including secret material.
func decodeResponse(r io.Reader, into any) error {
	if err := json.NewDecoder(io.LimitReader(r, maxResponseBytes+1)).Decode(into); err != nil {
		return errors.New("malformed API response shape")
	}
	return nil
}

func validCreateID(id string) bool {
	return id != "" && len(id) <= 512 && strings.TrimSpace(id) == id && !strings.ContainsAny(id, "/?#\\") && strings.IndexFunc(id, unicode.IsControl) < 0
}
