package store

import (
	"encoding/json"
	"testing"
)

func TestNullStringJSONRoundTrip(t *testing.T) {
	cases := []NullString{
		{String: "", Valid: false},
		{String: "hello", Valid: true},
		{String: "", Valid: true}, // valid empty string, distinct from absent
	}

	for _, want := range cases {
		data, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("Marshal(%+v): %v", want, err)
		}

		var got NullString
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if got != want {
			t.Errorf("round trip %+v -> %s -> %+v", want, data, got)
		}
	}
}

func TestNullStringMarshalsAbsentAsNull(t *testing.T) {
	data, err := json.Marshal(NullString{Valid: false})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("Marshal(absent) = %s, want null", data)
	}
}
