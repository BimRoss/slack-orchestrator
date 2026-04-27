package termsredis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestHumansTermsAccepted_missingIndex(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = rdb.Close() }()
	c := &Checker{rdb: rdb}
	ok, err := c.HumansTermsAccepted(context.Background(), "U999")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("expected false without index")
	}
}

func TestHumansTermsAccepted_accepted(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = rdb.Close() }()
	_ = rdb.Set(context.Background(), "makeacompany:user_by_slack:U1", "Pat@Example.com", 0).Err()
	_ = rdb.HSet(context.Background(), "makeacompany:user_profile:pat@example.com", "humans_terms_accepted_at", "2026-04-27T12:00:00Z").Err()

	c := &Checker{rdb: rdb}
	ok, err := c.HumansTermsAccepted(context.Background(), "U1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected true when timestamp set")
	}
}

func TestHumansTermsAccepted_notAccepted(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = rdb.Close() }()
	_ = rdb.Set(context.Background(), "makeacompany:user_by_slack:U2", "x@y.co", 0).Err()
	_ = rdb.HSet(context.Background(), "makeacompany:user_profile:x@y.co", "email", "x@y.co").Err()

	c := &Checker{rdb: rdb}
	ok, err := c.HumansTermsAccepted(context.Background(), "U2")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("expected false when humans_terms_accepted_at missing")
	}
}

func TestNewCheckerFromURL_invalid(t *testing.T) {
	_, err := NewCheckerFromURL("not-a-url")
	if err == nil {
		t.Fatal("expected error")
	}
}
