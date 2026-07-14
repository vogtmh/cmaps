package store

import (
	"fmt"
	"io"
	"time"

	bolt "go.etcd.io/bbolt"
)

// This file provides bucket-level primitives used by the web layer's identifier
// migration engine and the backup export/import, so those handlers never need
// raw access to the underlying bolt transactions.

// CountBucket returns the number of keys in a bucket (0 if the bucket is absent).
func (db *DB) CountBucket(bucket []byte) int {
	n := 0
	_ = db.bolt.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket(bucket); b != nil {
			n = b.Stats().KeyN
		}
		return nil
	})
	return n
}

// DropBuckets deletes the named buckets, ignoring any that do not exist.
func (db *DB) DropBuckets(buckets ...[]byte) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		for _, b := range buckets {
			_ = tx.DeleteBucket(b)
		}
		return nil
	})
}

// GetRaw returns the raw value stored at bucket/key.
func (db *DB) GetRaw(bucket, key []byte) ([]byte, bool, error) {
	var out []byte
	found := false
	err := db.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		v := b.Get(key)
		if v == nil {
			return nil
		}
		out = append([]byte(nil), v...)
		found = true
		return nil
	})
	return out, found, err
}

// PutRaw stores a raw value at bucket/key, creating the bucket if needed.
func (db *DB) PutRaw(bucket, key, val []byte) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return err
		}
		return b.Put(key, val)
	})
}

// StageTransform recreates the dst bucket, scans every row of src and, for each
// row, calls fn(k, v). When keep is true the returned value is written into dst
// under the same key; total counts every row for which counted is true (letting
// callers exclude rows they never intend to stage, e.g. local users), and
// changed counts rows actually staged.
func (db *DB) StageTransform(dst, src []byte, fn func(k, v []byte) (newVal []byte, keep, counted bool)) (total, changed int, err error) {
	err = db.bolt.Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket(dst)
		d, e := tx.CreateBucket(dst)
		if e != nil {
			return e
		}
		s := tx.Bucket(src)
		if s == nil {
			return nil
		}
		c := s.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			nv, keep, counted := fn(append([]byte(nil), k...), append([]byte(nil), v...))
			if counted {
				total++
			}
			if !keep {
				continue
			}
			if e := d.Put(append([]byte(nil), k...), nv); e != nil {
				return e
			}
			changed++
		}
		return nil
	})
	return
}

// ApplyStaged copies every row from the stage bucket into the live bucket. The
// destination key is keyOf(k, v); when it differs from the staged key the old
// key is deleted (a re-key). Rows for which keyOf reports ok=false are skipped.
// Returns the number of rows applied.
func (db *DB) ApplyStaged(live, stage []byte, keyOf func(k, v []byte) ([]byte, bool)) (int, error) {
	n := 0
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		s := tx.Bucket(stage)
		if s == nil {
			return nil
		}
		l, err := tx.CreateBucketIfNotExists(live)
		if err != nil {
			return err
		}
		c := s.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			key := append([]byte(nil), k...)
			val := append([]byte(nil), v...)
			newKey, ok := keyOf(key, val)
			if !ok {
				continue
			}
			if e := l.Put(newKey, val); e != nil {
				return e
			}
			if string(newKey) != string(key) {
				if e := l.Delete(key); e != nil {
					return e
				}
			}
			n++
		}
		return nil
	})
	return n, err
}

// SnapshotTo writes a consistent snapshot of the entire database to w.
func (db *DB) SnapshotTo(w io.Writer) error {
	return db.bolt.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(w)
		return err
	})
}

// ReplaceBucketsFromFile opens the bolt database at path (read-only) and
// replaces each named bucket in db with the archived contents: an exact
// overwrite, never a merge, so a bucket missing from the archive is cleared.
// Returns the number of rows written.
func (db *DB) ReplaceBucketsFromFile(path string, buckets [][]byte) (int, error) {
	src, err := bolt.Open(path, 0600, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
	if err != nil {
		return 0, fmt.Errorf("open archive: %w", err)
	}
	defer src.Close()

	// Read all source entries into memory first so we never hold a read tx on
	// the source while writing the destination.
	type kv struct{ k, v []byte }
	staged := make(map[string][]kv, len(buckets))
	if err := src.View(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			b := tx.Bucket(name)
			if b == nil {
				staged[string(name)] = nil
				continue
			}
			var rows []kv
			_ = b.ForEach(func(k, v []byte) error {
				if v == nil {
					return nil // nested buckets are not used in this schema
				}
				rows = append(rows, kv{append([]byte(nil), k...), append([]byte(nil), v...)})
				return nil
			})
			staged[string(name)] = rows
		}
		return nil
	}); err != nil {
		return 0, err
	}

	count := 0
	if err := db.bolt.Update(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			if err := tx.DeleteBucket(name); err != nil && err != bolt.ErrBucketNotFound {
				return err
			}
			nb, err := tx.CreateBucket(name)
			if err != nil {
				return err
			}
			for _, row := range staged[string(name)] {
				if err := nb.Put(row.k, row.v); err != nil {
					return err
				}
				count++
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}
