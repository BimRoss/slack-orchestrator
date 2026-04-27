package threadpin

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestStoreSetGetRoundTrip(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	st, err := NewStoreFromURL("redis://" + srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	if err := st.SetFollowupEmployee(ctx, "T1", "C1", "177.1", "joanne"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetFollowupEmployee(ctx, "T1", "C1", "177.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "joanne" {
		t.Fatalf("got %q want joanne", got)
	}
}

func TestStoreGetMissing(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	st, err := NewStoreFromURL("redis://" + srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	got, err := st.GetFollowupEmployee(context.Background(), "T1", "C1", "999.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("want empty got %q", got)
	}
}
