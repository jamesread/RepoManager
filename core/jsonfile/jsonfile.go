// Package jsonfile reads and writes pretty-printed JSON files atomically.
package jsonfile

import (
	"encoding/json"
	"os"
)

// Read unmarshals the JSON file at path into dest.
func Read(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Write marshals v with indentation and replaces the file at path
// atomically via a temporary sibling file.
func Write(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
