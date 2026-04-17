package config

import (
	"reflect"
	"testing"
)

func TestParseBotUserMap_positional(t *testing.T) {
	got := parseBotUserMap("U1,U2,U3")
	want := map[string]string{"U1": "alex", "U2": "tim", "U3": "ross"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseBotUserMap_explicit(t *testing.T) {
	got := parseBotUserMap("garth=UG1,alex=UA1")
	want := map[string]string{"UG1": "garth", "UA1": "alex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

