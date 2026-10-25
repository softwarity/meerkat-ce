// Package jsonbody decodes a request's JSON body strictly, and says where it
// breaks.
//
// encoding/json reports a malformed document by what it found ("invalid
// character '"' after object key:value pair") and not by where: an
// integrator looking at forty lines for a missing comma is told nothing
// that narrows it down. The decoder knows the byte it stopped at, so the
// error names the line and the column.
package jsonbody

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Decode reads r whole and decodes it into v, refusing unknown fields.
func Decode(r io.Reader, v any) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Locate(raw, err)
	}
	return nil
}

// Locate prefixes a decoding error with the line and column it happened at,
// when the error carries a position; any other error comes back as it was.
func Locate(raw []byte, err error) error {
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	offset := int64(-1)
	switch {
	case errors.As(err, &syntax):
		// The offset counts the byte that broke the document: the column
		// names that byte, not the one after it.
		offset = syntax.Offset - 1
	case errors.As(err, &typ):
		offset = typ.Offset - 1
	}
	if offset < 0 || offset > int64(len(raw)) {
		return err
	}
	line, col := 1, 1
	for _, b := range raw[:offset] {
		if b == '\n' {
			line, col = line+1, 1
			continue
		}
		col++
	}
	return fmt.Errorf("line %d, column %d: %w", line, col, err)
}
