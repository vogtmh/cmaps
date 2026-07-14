package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCountAndRawPrimitives(t *testing.T) {
	db := newTestDB(t)
	bkt := []byte("migstage_test")

	if n := db.CountBucket(bkt); n != 0 {
		t.Errorf("CountBucket on absent bucket = %d, want 0", n)
	}
	if err := db.PutRaw(bkt, []byte("a"), []byte("1")); err != nil {
		t.Fatalf("PutRaw: %v", err)
	}
	if err := db.PutRaw(bkt, []byte("b"), []byte("2")); err != nil {
		t.Fatalf("PutRaw: %v", err)
	}
	if n := db.CountBucket(bkt); n != 2 {
		t.Errorf("CountBucket = %d, want 2", n)
	}
	v, found, err := db.GetRaw(bkt, []byte("a"))
	if err != nil || !found || string(v) != "1" {
		t.Errorf("GetRaw(a) = %q found=%v err=%v, want \"1\"", v, found, err)
	}
	if _, found, _ := db.GetRaw(bkt, []byte("missing")); found {
		t.Errorf("GetRaw(missing) reported found")
	}
	if err := db.DropBuckets(bkt); err != nil {
		t.Fatalf("DropBuckets: %v", err)
	}
	if n := db.CountBucket(bkt); n != 0 {
		t.Errorf("CountBucket after drop = %d, want 0", n)
	}
}

func TestStageTransformAndApply(t *testing.T) {
	db := newTestDB(t)
	src := []byte("stage_src")
	dst := []byte("stage_dst")
	for _, k := range []string{"k1", "k2", "k3"} {
		if err := db.PutRaw(src, []byte(k), []byte("v-"+k)); err != nil {
			t.Fatalf("PutRaw: %v", err)
		}
	}

	// Stage: keep k1 and k3 (changed), skip k2; all counted.
	total, changed, err := db.StageTransform(dst, src, func(k, v []byte) ([]byte, bool, bool) {
		if string(k) == "k2" {
			return nil, false, true
		}
		return append([]byte("new-"), v...), true, true
	})
	if err != nil {
		t.Fatalf("StageTransform: %v", err)
	}
	if total != 3 || changed != 2 {
		t.Errorf("StageTransform total=%d changed=%d, want 3/2", total, changed)
	}
	if n := db.CountBucket(dst); n != 2 {
		t.Errorf("dst count = %d, want 2", n)
	}
	if v, _, _ := db.GetRaw(dst, []byte("k1")); string(v) != "new-v-k1" {
		t.Errorf("staged k1 = %q, want new-v-k1", v)
	}

	// counted=false rows are excluded from total (mimics local-user skip).
	total2, _, err := db.StageTransform([]byte("stage_dst2"), src, func(k, v []byte) ([]byte, bool, bool) {
		if string(k) == "k1" {
			return nil, false, false // not counted
		}
		return v, false, true
	})
	if err != nil {
		t.Fatalf("StageTransform2: %v", err)
	}
	if total2 != 2 {
		t.Errorf("StageTransform2 total=%d, want 2 (k1 excluded)", total2)
	}

	// Apply staged dst into a live bucket, re-keying k1 -> K1.
	live := []byte("stage_live")
	applied, err := db.ApplyStaged(live, dst, func(k, v []byte) ([]byte, bool) {
		if string(k) == "k1" {
			return []byte("K1"), true
		}
		return k, true
	})
	if err != nil {
		t.Fatalf("ApplyStaged: %v", err)
	}
	if applied != 2 {
		t.Errorf("ApplyStaged applied=%d, want 2", applied)
	}
	if v, found, _ := db.GetRaw(live, []byte("K1")); !found || string(v) != "new-v-k1" {
		t.Errorf("re-keyed K1 = %q found=%v, want new-v-k1", v, found)
	}
	if _, found, _ := db.GetRaw(live, []byte("k1")); found {
		t.Errorf("old key k1 still present after re-key")
	}
	if v, found, _ := db.GetRaw(live, []byte("k3")); !found || string(v) != "new-v-k3" {
		t.Errorf("k3 = %q found=%v, want new-v-k3", v, found)
	}
}

func TestSnapshotAndReplaceBucketsFromFile(t *testing.T) {
	src := newTestDB(t)
	if err := src.PutMap(MapInfo{Mapname: "hq", DisplayName: "HQ"}); err != nil {
		t.Fatalf("PutMap: %v", err)
	}
	if err := src.PutUser(User{Username: "jdoe", Role: 1}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}

	// Snapshot the source DB to a file.
	snapPath := filepath.Join(t.TempDir(), "snap.db")
	var buf bytes.Buffer
	if err := src.SnapshotTo(&buf); err != nil {
		t.Fatalf("SnapshotTo: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("SnapshotTo produced empty output")
	}
	if err := os.WriteFile(snapPath, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	// Restore just the maps bucket into a fresh DB.
	dst := newTestDB(t)
	n, err := dst.ReplaceBucketsFromFile(snapPath, [][]byte{BucketMaps})
	if err != nil {
		t.Fatalf("ReplaceBucketsFromFile: %v", err)
	}
	if n != 1 {
		t.Errorf("restored rows = %d, want 1", n)
	}
	m, found, _ := dst.GetMap("hq")
	if !found || m.DisplayName != "HQ" {
		t.Errorf("restored map = %+v found=%v", m, found)
	}
	// Users bucket was not selected, so it must remain empty.
	if users, _ := dst.ListUsers(); len(users) != 0 {
		t.Errorf("unselected users bucket was populated: %d", len(users))
	}
}
