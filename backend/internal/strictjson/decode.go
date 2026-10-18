// Package strictjson rejects ambiguous objects and unsupported fields before a
// typed contract can be used for state changes or execution control.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func Decode(data []byte, target any) error {
	if err := uniqueKeys(json.NewDecoder(bytes.NewReader(data)), 0); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing JSON content")
	}
	return nil
}

func uniqueKeys(decoder *json.Decoder, depth int) error {
	if depth > 100 {
		return fmt.Errorf("JSON nesting exceeds limits")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid JSON object key")
			}
			seen[key] = true
			if err := uniqueKeys(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueKeys(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}
