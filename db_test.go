package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

// newTestDB opens a fresh bolt database in a temp dir, closed automatically
// when the test finishes.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestGetPutJSONRoundtrip(t *testing.T) {
	db := newTestDB(t)

	// Missing key -> found=false, zero value.
	got, found, err := getJSON[MapInfo](db, bucketMaps, []byte("nope"))
	if err != nil {
		t.Fatalf("getJSON missing key: %v", err)
	}
	if found {
		t.Errorf("getJSON reported found for a missing key")
	}
	if got.Mapname != "" {
		t.Errorf("getJSON returned non-zero value for missing key: %+v", got)
	}

	want := MapInfo{Mapname: "hq", DisplayName: "Headquarters", Lat: 48.1, Lon: 11.5, MapX: 10, MapY: 20}
	if err := putJSON(db, bucketMaps, []byte("hq"), want); err != nil {
		t.Fatalf("putJSON: %v", err)
	}
	got, found, err = getJSON[MapInfo](db, bucketMaps, []byte("hq"))
	if err != nil || !found {
		t.Fatalf("getJSON after put: found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roundtrip mismatch:\n got %+v\nwant %+v", got, want)
	}

	// Overwrite in place.
	want.DisplayName = "HQ Munich"
	if err := putJSON(db, bucketMaps, []byte("hq"), want); err != nil {
		t.Fatalf("putJSON overwrite: %v", err)
	}
	got, _, _ = getJSON[MapInfo](db, bucketMaps, []byte("hq"))
	if got.DisplayName != "HQ Munich" {
		t.Errorf("overwrite not persisted, got %q", got.DisplayName)
	}
}

func TestDeleteKey(t *testing.T) {
	db := newTestDB(t)
	if err := putJSON(db, bucketSettings, []byte("k"), "v"); err != nil {
		t.Fatalf("putJSON: %v", err)
	}
	if err := deleteKey(db, bucketSettings, []byte("k")); err != nil {
		t.Fatalf("deleteKey: %v", err)
	}
	_, found, err := getJSON[string](db, bucketSettings, []byte("k"))
	if err != nil {
		t.Fatalf("getJSON after delete: %v", err)
	}
	if found {
		t.Errorf("key still present after deleteKey")
	}
	// Deleting a missing key must not error.
	if err := deleteKey(db, bucketSettings, []byte("missing")); err != nil {
		t.Errorf("deleteKey on missing key: %v", err)
	}
}

func TestListJSONPrefix(t *testing.T) {
	db := newTestDB(t)
	desks := []Desk{
		{ID: 1, Map: "hq", Desktype: "addesk", Desknumber: "A1"},
		{ID: 2, Map: "hq", Desktype: "addesk", Desknumber: "A2"},
		{ID: 1, Map: "lab", Desktype: "hotseat", Desknumber: "L1"},
	}
	for _, d := range desks {
		if err := putJSON(db, bucketDesks, deskKey(d.Map, d.ID), d); err != nil {
			t.Fatalf("putJSON desk: %v", err)
		}
	}

	all, err := listJSON[Desk](db, bucketDesks, "")
	if err != nil {
		t.Fatalf("listJSON all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("listJSON all: got %d entries, want 3", len(all))
	}

	hq, err := listJSON[Desk](db, bucketDesks, "hq:")
	if err != nil {
		t.Fatalf("listJSON prefix: %v", err)
	}
	if len(hq) != 2 {
		t.Fatalf("listJSON prefix hq: got %d entries, want 2", len(hq))
	}
	for _, d := range hq {
		if d.Map != "hq" {
			t.Errorf("prefix scan returned desk from map %q", d.Map)
		}
	}

	none, err := listJSON[Desk](db, bucketDesks, "zzz:")
	if err != nil {
		t.Fatalf("listJSON empty prefix result: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("listJSON with non-matching prefix returned %d entries", len(none))
	}
}

func TestSeqKeyOrdering(t *testing.T) {
	// Big-endian encoding must preserve numeric ordering byte-wise, which the
	// audit log and bookings rely on for cursor iteration order.
	prev := seqKey(0)
	for _, n := range []uint64{1, 2, 255, 256, 65535, 1 << 20, 1 << 40} {
		cur := seqKey(n)
		if len(cur) != 8 {
			t.Fatalf("seqKey(%d) length = %d, want 8", n, len(cur))
		}
		if string(prev) >= string(cur) {
			t.Errorf("seqKey ordering broken: key(%d) not > previous", n)
		}
		prev = cur
	}
}

func TestUserRoundtripNormalizesMail(t *testing.T) {
	db := newTestDB(t)
	u := User{Username: "jdoe", Role: 1, Mail: "John.Doe@Example.COM", Fullname: "John Doe"}
	if err := db.PutUser(u); err != nil {
		t.Fatalf("PutUser: %v", err)
	}
	got, found, err := db.GetUser("jdoe")
	if err != nil || !found {
		t.Fatalf("GetUser: found=%v err=%v", found, err)
	}
	if got.Mail != "john.doe@example.com" {
		t.Errorf("PutUser did not lowercase mail: %q", got.Mail)
	}
}
