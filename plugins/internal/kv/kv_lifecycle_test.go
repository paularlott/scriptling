package kv

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/plugins/internal/relational"
)

// The KV client wraps a Store that owns an OS resource: a network connection
// (valkey) or, for badgerdb, an embedded database that holds an exclusive OS
// lock on its directory — badgerdb.Open fails if the same directory is opened
// twice in a process. A server that builds a fresh interpreter per request,
// opens a client, and returns without calling close() must still release that
// resource when the instance is collected, or the badgerdb directory stays
// locked against every later open().
//
// The relational Connection class installs a __del__ finalizer for exactly
// this reason. This test asserts the KV client class does the same, so the two
// shared plugin kits stay symmetric. It currently fails: kv.ClientClass has
// close() but no __del__.

// countingStore is a Store whose data methods are no-ops; the test only drives
// class construction and lifecycle method presence.
type countingStore struct{ closed int }

func (s *countingStore) Get(context.Context, string) (string, bool, error)    { return "", false, nil }
func (s *countingStore) Set(context.Context, string, string, int64) error     { return nil }
func (s *countingStore) Delete(context.Context, []string) (int64, error)      { return 0, nil }
func (s *countingStore) Exists(context.Context, []string) (int64, error)      { return 0, nil }
func (s *countingStore) Expire(context.Context, string, int64) (bool, error)  { return false, nil }
func (s *countingStore) Persist(context.Context, string) (bool, error)        { return false, nil }
func (s *countingStore) TTL(context.Context, string) (int64, bool, error)     { return 0, false, nil }
func (s *countingStore) Incr(context.Context, string, int64) (int64, error)   { return 0, nil }
func (s *countingStore) Keys(context.Context, string) ([]string, error)       { return nil, nil }
func (s *countingStore) MGet(context.Context, []string) ([]*string, error)    { return nil, nil }
func (s *countingStore) MSet(context.Context, map[string]string, int64) error { return nil }
func (s *countingStore) SetNX(context.Context, string, string, int64) (bool, error) {
	return false, nil
}
func (s *countingStore) HashSet(context.Context, string, string, string) (int64, error) {
	return 0, nil
}
func (s *countingStore) HashGet(context.Context, string, string) (*string, error) { return nil, nil }
func (s *countingStore) HashDelete(context.Context, string, []string) (int64, error) {
	return 0, nil
}
func (s *countingStore) HashAll(context.Context, string) (map[string]string, error) {
	return nil, nil
}
func (s *countingStore) HashSize(context.Context, string) (int64, error) { return 0, nil }
func (s *countingStore) Ping(context.Context) error                      { return nil }
func (s *countingStore) Close() error                                    { s.closed++; return nil }

// TestKVClientHasDelFinalizer guards that the KV client class registers a
// __del__ finalizer, matching relational.Connection, so an abandoned
// valkey/badgerdb client releases its handle on collection.
func TestKVClientHasDelFinalizer(t *testing.T) {
	class := ClientClass(func() (*Client, error) {
		return &Client{Store: &countingStore{}}, nil
	}, nil).Build()

	if _, ok := class.Methods["close"]; !ok {
		t.Fatal("KV client class is missing close(); test premise is wrong")
	}
	if _, ok := class.Methods["__del__"]; !ok {
		t.Fatal("KV client class has no __del__ finalizer: an abandoned valkey/badgerdb " +
			"client leaks its handle (and for badgerdb keeps the directory lock) until the " +
			"process exits. relational.Connection registers __del__ for this; the KV kit should too.")
	}
}

// TestRelationalConnectionHasDelFinalizer is the control: it confirms the
// symmetric kit already has the finalizer, so the KV expectation above is
// consistent with the codebase rather than a new demand.
func TestRelationalConnectionHasDelFinalizer(t *testing.T) {
	class := relational.ConnectionClass(func() (*relational.Conn, error) {
		return &relational.Conn{}, nil
	}).Build()
	if _, ok := class.Methods["__del__"]; !ok {
		t.Fatal("expected relational.Connection to register __del__ (control assertion)")
	}
}
