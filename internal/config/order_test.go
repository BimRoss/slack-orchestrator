package config

import (
	"reflect"
	"testing"
)

func TestResolveMultiagentOrder_explicitOverride(t *testing.T) {
	got := ResolveMultiagentOrder(
		[]string{"tim", "alex"},
		map[string]string{"U1": "garth"},
		"secret",
	)
	want := []string{"tim", "alex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveMultiagentOrder_derivedShuffleStable(t *testing.T) {
	m := map[string]string{
		"U1": "alex",
		"U2": "tim",
		"U3": "ross",
	}
	a := ResolveMultiagentOrder(nil, m, "prod-secret-1")
	b := ResolveMultiagentOrder(nil, m, "prod-secret-1")
	if len(a) != 3 || !reflect.DeepEqual(a, b) {
		t.Fatalf("stable roster: a=%v b=%v", a, b)
	}
	c := ResolveMultiagentOrder(nil, m, "prod-secret-2")
	if len(c) != 3 {
		t.Fatalf("len(c)=%d", len(c))
	}
}

func TestResolveMultiagentOrder_emptyMap(t *testing.T) {
	if got := ResolveMultiagentOrder(nil, nil, "x"); got != nil {
		t.Fatalf("got %v want nil", got)
	}
}

func TestDerivedShuffleSeed_stable(t *testing.T) {
	m := map[string]string{"U2": "tim", "U1": "alex"}
	a := DerivedShuffleSeed(m)
	b := DerivedShuffleSeed(m)
	if a != b || len(a) != 64 {
		t.Fatalf("seed=%q len=%d", a, len(a))
	}
}
