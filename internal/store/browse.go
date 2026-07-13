package store

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	bolt "go.etcd.io/bbolt"
)

// The database browser is a read-only view of the BoltDB store, exposed under
// the admin panel's Sync tab. It never writes to the database (View only) and
// redacts obviously sensitive values (password hashes, salts, bind passwords,
// API tokens, secrets) so that a read-level admin cannot harvest credentials.

const dbRedactMarker = "\u2022\u2022\u2022 redacted \u2022\u2022\u2022"

// BucketInfo is one bucket summary for the browser sidebar.
type BucketInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// BrowseEntry is a single key/value pair inside a bucket.
type BrowseEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// dbSensitiveHints are substrings (matched case-insensitively) that mark a JSON
// field name or bucket key as holding a secret that must be redacted.
var dbSensitiveHints = []string{
	"password", "passhash", "pass_hash", "passwd", "salt",
	"token", "secret", "apikey", "api_key",
	"privatekey", "private_key", "client_secret",
}

func dbIsSensitiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, h := range dbSensitiveHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// dbDecodeKey renders a raw Bolt key as a readable string. Sequence buckets use
// 8-byte big-endian keys, which would otherwise show as control characters.
func dbDecodeKey(k []byte) string {
	if len(k) == 8 {
		printable := true
		for _, b := range k {
			if b < 0x20 || b > 0x7e {
				printable = false
				break
			}
		}
		if !printable {
			return "#" + strconv.FormatUint(binary.BigEndian.Uint64(k), 10)
		}
	}
	if utf8.Valid(k) {
		return string(k)
	}
	return fmt.Sprintf("%x", k)
}

// dbRedactValue redacts secrets in a stored value. Whole values keyed by a
// sensitive name (e.g. robinconfig "token") are replaced entirely; JSON objects
// have their sensitive fields blanked while keeping the rest visible.
func dbRedactValue(key, val string) string {
	if dbIsSensitiveName(key) {
		return dbRedactMarker
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(val), &obj) == nil && obj != nil {
		changed := false
		for k := range obj {
			if dbIsSensitiveName(k) {
				obj[k] = json.RawMessage(`"` + dbRedactMarker + `"`)
				changed = true
			}
		}
		if changed {
			if b, err := json.Marshal(obj); err == nil {
				return string(b)
			}
		}
	}
	return val
}

// BrowseBuckets returns every bucket with its key count, sorted by name.
func (db *DB) BrowseBuckets() ([]BucketInfo, error) {
	var out []BucketInfo
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			out = append(out, BucketInfo{Name: string(name), Count: b.Stats().KeyN})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// BrowseBucket returns a page of key/value entries from a bucket. When search is
// non-empty, only entries whose key or (raw) value contains it are counted and
// returned. total is the number of matching entries across the whole bucket.
func (db *DB) BrowseBucket(name, search string, offset, limit int) (entries []BrowseEntry, total int, err error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	search = strings.ToLower(search)
	entries = []BrowseEntry{}
	err = db.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				continue // skip nested buckets
			}
			keyStr := dbDecodeKey(k)
			valStr := string(v)
			if search != "" {
				if !strings.Contains(strings.ToLower(keyStr), search) &&
					!strings.Contains(strings.ToLower(valStr), search) {
					continue
				}
			}
			if total >= offset && len(entries) < limit {
				entries = append(entries, BrowseEntry{Key: keyStr, Value: dbRedactValue(keyStr, valStr)})
			}
			total++
		}
		return nil
	})
	return entries, total, err
}
