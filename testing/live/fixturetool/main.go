// fixturetool transforms only explicitly owned synthetic fixture exports.
// Never pass token files or pre-existing workspace resource exports to it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

func jsonValue(v interface{}) (interface{}, error) {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		out := map[string]interface{}{}
		for k, value := range x {
			key, ok := k.(string)
			if !ok {
				return nil, errors.New("non-string document key")
			}
			normalized, err := jsonValue(value)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, value := range x {
			normalized, err := jsonValue(value)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	case string:
		if strings.Contains(x, "<REDACTED>") {
			return nil, errors.New("withheld credential placeholder cannot be cloned")
		}
		return x, nil
	default:
		return x, nil
	}
}

func cloneResource(input []byte, oldName, newName string) (map[string]interface{}, error) {
	var raw interface{}
	if yaml.Unmarshal(input, &raw) != nil {
		return nil, errors.New("invalid resource export")
	}
	normalized, err := jsonValue(raw)
	if err != nil {
		return nil, err
	}
	doc, ok := normalized.(map[string]interface{})
	if !ok || doc["type"] != "resource" {
		return nil, errors.New("expected one resource document")
	}
	cfg, ok := doc["configuration"].(map[string]interface{})
	if !ok || cfg["name"] != oldName {
		return nil, errors.New("export identity mismatch")
	}
	typ, ok := cfg["type"].(string)
	if !ok || typ == "" {
		return nil, errors.New("missing integration type")
	}
	if oldName == newName || !strings.HasPrefix(oldName, "tflive-") || !strings.HasPrefix(newName, "tflive-") {
		return nil, errors.New("identities must be distinct owned fixtures")
	}
	cfg["name"] = newName
	return doc, nil
}

func writePrivate(path string, value map[string]interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return errors.New("cannot encode fixture")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("cannot create new private fixture output")
	}
	_, writeErr := f.Write(append(data, '\n'))
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return errors.New("cannot persist fixture output")
	}
	return nil
}

func run(args []string) error {
	if len(args) != 5 || args[0] != "clone-resource" {
		return errors.New("usage: fixturetool clone-resource INPUT OLD_NAME NEW_NAME NEW_OUTPUT")
	}
	input, err := os.ReadFile(args[1])
	if err != nil {
		return errors.New("cannot read owned export")
	}
	value, err := cloneResource(input, args[2], args[3])
	if err != nil {
		return err
	}
	return writePrivate(args[4], value)
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("private clone document written")
}
