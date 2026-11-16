package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClonePreservesSecretAndUnknownFields(t *testing.T) {
	input := []byte("type: resource\nconfiguration:\n  name: tflive-old\n  type: postgres\n  password: synthetic-canary\nuserTags: [phase=create]\nextra:\n  preserved: true\n")
	got, err := cloneResource(input, "tflive-old", "tflive-new")
	if err != nil {
		t.Fatal(err)
	}
	if got["configuration"].(map[string]interface{})["password"] != "synthetic-canary" {
		t.Fatal("credential changed")
	}
	if got["configuration"].(map[string]interface{})["name"] != "tflive-new" {
		t.Fatal("identity not changed")
	}
	if got["extra"].(map[string]interface{})["preserved"] != true {
		t.Fatal("unknown fields lost")
	}
}
func TestCloneRejectsWrongIdentityAndRedaction(t *testing.T) {
	for _, input := range []string{
		"type: resource\nconfiguration: {name: other, type: postgres, password: synthetic-canary}",
		"type: resource\nconfiguration: {name: tflive-old, type: postgres, password: '<REDACTED>'}",
		"type: group\nconfiguration: {name: tflive-old}",
		"type: resource\nconfiguration: [malformed]",
	} {
		if _, err := cloneResource([]byte(input), "tflive-old", "tflive-new"); err == nil {
			t.Fatal("unsafe export accepted")
		}
	}
}
func TestPrivateOutputDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := writePrivate(path, map[string]interface{}{"test": true}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatal("output permissions not private")
	}
	if err := writePrivate(path, map[string]interface{}{}); err == nil {
		t.Fatal("existing evidence overwritten")
	}
}
