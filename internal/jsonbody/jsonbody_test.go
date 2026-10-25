package jsonbody

import (
	"strings"
	"testing"
)

// The missing comma an integrator actually sent: the error names its line.
func TestAMalformedBodySaysWhere(t *testing.T) {
	body := "{\n  \"tenant\": \"Default\"\n  \"title\": \"Notification\"\n}"
	var v map[string]any
	err := Decode(strings.NewReader(body), &v)
	if err == nil || !strings.HasPrefix(err.Error(), "line 3, column 3: ") {
		t.Fatalf("got %v, want the error at line 3, column 3", err)
	}
}

func TestAWrongTypeSaysWhere(t *testing.T) {
	var v struct {
		N int `json:"n"`
	}
	err := Decode(strings.NewReader("{\n\"n\": \"x\"}"), &v)
	if err == nil || !strings.HasPrefix(err.Error(), "line 2, ") {
		t.Fatalf("got %v, want line 2", err)
	}
}

func TestAnUnknownFieldIsRefused(t *testing.T) {
	var v struct {
		N int `json:"n"`
	}
	if err := Decode(strings.NewReader(`{"m":1}`), &v); err == nil {
		t.Fatal("an unknown field was accepted")
	}
}
