package components

import "testing"

func TestSameStringSetIgnoresOrderAndDuplicates(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{}, nil, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a", "a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a"}, []string{"A"}, false},
	}
	for _, c := range cases {
		if got := sameStringSet(c.a, c.b); got != c.want {
			t.Errorf("sameStringSet(%v, %v) = %v", c.a, c.b, got)
		}
	}
}

// The backend lowercases weekday names, so "Monday" and "monday" are the same
// membership and must not read as drift.
func TestSameStringSetFoldIgnoresCase(t *testing.T) {
	if !sameStringSetFold([]string{"Monday", "Tuesday"}, []string{"tuesday", "monday"}) {
		t.Error("case-different weekday names should compare equal")
	}
	if sameStringSetFold([]string{"Monday"}, []string{"monday", "friday"}) {
		t.Error("a genuinely different set must not compare equal")
	}
}

func TestSameInstantSetComparesMoments(t *testing.T) {
	if !sameInstantSet([]string{"2030-01-01T05:30:00+05:30"}, []string{"2030-01-01T00:00:00Z"}) {
		t.Error("the same instant written in different offsets should compare equal")
	}
	if sameInstantSet([]string{"2030-01-01T00:00:00Z"}, []string{"2030-01-02T00:00:00Z"}) {
		t.Error("different instants must not compare equal")
	}
	// Unparseable values fall back to exact string comparison rather than
	// collapsing to equal.
	if !sameInstantSet([]string{"not-a-date"}, []string{"not-a-date"}) {
		t.Error("identical unparseable values should compare equal")
	}
	if sameInstantSet([]string{"not-a-date"}, []string{"other"}) {
		t.Error("different unparseable values must not compare equal")
	}
}

func TestSetPreservingOrder(t *testing.T) {
	prior := []string{"b", "a"}
	if got := setPreservingOrder(prior, []string{"a", "b"}); got[0] != "b" {
		t.Errorf("equal sets should keep the configured ordering, got %v", got)
	}
	if got := setPreservingOrder(prior, []string{"c"}); len(got) != 1 || got[0] != "c" {
		t.Errorf("a changed set should take the server value, got %v", got)
	}
}

// An optional attribute the configuration never set must not be populated from
// the server: adopting a server-side default would show a diff on every plan.
func TestOptStringRefreshPolicy(t *testing.T) {
	if got := optString("", "server", false); got != "" {
		t.Errorf("unset attribute should stay unset, got %q", got)
	}
	if got := optString("mine", "server", false); got != "server" {
		t.Errorf("a set attribute should follow the server, got %q", got)
	}
	if got := optString("mine", "", false); got != "" {
		t.Errorf("a cleared server value is drift too, got %q", got)
	}
	if got := optString("", "server", true); got != "server" {
		t.Errorf("import should take the server value, got %q", got)
	}
}

func TestSameStringMap(t *testing.T) {
	if !sameStringMap(nil, nil) {
		t.Error("two empty maps should compare equal")
	}
	if !sameStringMap(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
		t.Error("identical maps should compare equal")
	}
	if sameStringMap(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Error("a changed value must not compare equal")
	}
	if sameStringMap(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}) {
		t.Error("an added key must not compare equal")
	}
}
