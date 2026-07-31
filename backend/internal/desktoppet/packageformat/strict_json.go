package packageformat

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	maxStrictJSONBytes = 10 * 1024 * 1024
	maxStrictJSONDepth = 100
)

func DecodeStrictJSON(data []byte, target interface{}) error {
	if len(data) > maxStrictJSONBytes {
		return NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("json payload exceeds %d bytes", maxStrictJSONBytes), nil)
	}

	if err := checkDuplicateKeysAndDepth(data); err != nil {
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(target); err != nil {
		return NewPackageError(ErrCodePackageJsonUnknownField, "strict json decode failed", err)
	}

	if dec.More() {
		return NewPackageError(ErrCodePackageManifestInvalid, "unexpected trailing data after json value", nil)
	}

	return nil
}

func checkDuplicateKeysAndDepth(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return err
	}
	return scanValue(dec, tok, 0)
}

func scanValue(dec *json.Decoder, tok json.Token, depth int) error {
	if depth > maxStrictJSONDepth {
		return fmt.Errorf("json nesting depth exceeds %d", maxStrictJSONDepth)
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		keys := make(map[string]bool)
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyTok.(string)
			if !ok {
				return fmt.Errorf("expected string key in object")
			}
			if keys[key] {
				return NewPackageError(ErrCodePackageJsonDuplicateKey, fmt.Sprintf("duplicate key: %s", key), nil)
			}
			keys[key] = true

			valueTok, err := dec.Token()
			if err != nil {
				return err
			}
			if err := scanValue(dec, valueTok, depth+1); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return err
		}
	case '[':
		for dec.More() {
			elemTok, err := dec.Token()
			if err != nil {
				return err
			}
			if err := scanValue(dec, elemTok, depth+1); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return err
		}
	}

	return nil
}
