package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

// newTestDB opens a fresh bolt database in a temp dir, closed automatically
// when the test finishes.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestGetPutJSONRoundtrip(t *testing.T) {
	db := newTestDB(t)

	// Missing key -> found=false, zero value.
	got, found, err := GetJSON[MapInfo](db, BucketMaps, []byte("nope"))
	if err != nil {
		t.Fatalf("GetJSON missing key: %v", err)
	}
	if found {
		t.Errorf("GetJSON reported found for a missing key")
	}
	if got.Mapname != "" {
		t.Errorf("GetJSON returned non-zero value for missing key: %+v", got)
	}

	want := MapInfo{Mapname: "hq", DisplayName: "Headquarters", Lat: 48.1, Lon: 11.5, MapX: 10, MapY: 20}
	if err := PutJSON(db, BucketMaps, []byte("hq"), want); err != nil {
		t.Fatalf("PutJSON: %v", err)
	}
	got, found, err = GetJSON[MapInfo](db, BucketMaps, []byte("hq"))
	if err != nil || !found {
		t.Fatalf("GetJSON after put: found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roundtrip mismatch:\n got %+v\nwant %+v", got, want)
	}

	// Overwrite in place.
	want.DisplayName = "HQ Munich"
	if err := PutJSON(db, BucketMaps, []byte("hq"), want); err != nil {
		t.Fatalf("PutJSON overwrite: %v", err)
	}
	got, _, _ = GetJSON[MapInfo](db, BucketMaps, []byte("hq"))
	if got.DisplayName != "HQ Munich" {
		t.Errorf("overwrite not persisted, got %q", got.DisplayName)
	}
}

func TestDeleteKey(t *testing.T) {
	db := newTestDB(t)
	if err := PutJSON(db, BucketSettings, []byte("k"), "v"); err != nil {
		t.Fatalf("PutJSON: %v", err)
	}
	if err := DeleteKey(db, BucketSettings, []byte("k")); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}
	_, found, err := GetJSON[string](db, BucketSettings, []byte("k"))
	if err != nil {
		t.Fatalf("GetJSON after delete: %v", err)
	}
	if found {
		t.Errorf("key still present after DeleteKey")
	}
	// Deleting a missing key must not error.
	if err := DeleteKey(db, BucketSettings, []byte("missing")); err != nil {
		t.Errorf("DeleteKey on missing key: %v", err)
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
		if err := PutJSON(db, BucketDesks, DeskKey(d.Map, d.ID), d); err != nil {
			t.Fatalf("PutJSON desk: %v", err)
		}
	}

	all, err := ListJSON[Desk](db, BucketDesks, "")
	if err != nil {
		t.Fatalf("ListJSON all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListJSON all: got %d entries, want 3", len(all))
	}

	hq, err := ListJSON[Desk](db, BucketDesks, "hq:")
	if err != nil {
		t.Fatalf("ListJSON prefix: %v", err)
	}
	if len(hq) != 2 {
		t.Fatalf("ListJSON prefix hq: got %d entries, want 2", len(hq))
	}
	for _, d := range hq {
		if d.Map != "hq" {
			t.Errorf("prefix scan returned desk from map %q", d.Map)
		}
	}

	none, err := ListJSON[Desk](db, BucketDesks, "zzz:")
	if err != nil {
		t.Fatalf("ListJSON empty prefix result: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("ListJSON with non-matching prefix returned %d entries", len(none))
	}
}

func TestSeqKeyOrdering(t *testing.T) {
	// Big-endian encoding must preserve numeric ordering byte-wise, which the
	// audit log and bookings rely on for cursor iteration order.
	prev := SeqKey(0)
	for _, n := range []uint64{1, 2, 255, 256, 65535, 1 << 20, 1 << 40} {
		cur := SeqKey(n)
		if len(cur) != 8 {
			t.Fatalf("SeqKey(%d) length = %d, want 8", n, len(cur))
		}
		if string(prev) >= string(cur) {
			t.Errorf("SeqKey ordering broken: key(%d) not > previous", n)
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
