package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// boltDB buckets, replacing the MySQL tables of the PHP app. Per-map desk tables
// (desks_<map>) are collapsed into a single "desks" bucket keyed by "<map>:<id>".
var (
	bucketSettings  = []byte("settings")        // config_general (variable -> value)
	bucketMaps      = []byte("maplist")         // config_maplist (key = mapname)
	bucketDesks     = []byte("desks")           // desks_<map> (key = "<map>:<id>")
	bucketLdap      = []byte("ldapmirror")      // ldap-mirror (key = userid/ipphone)
	bucketDirectory = []byte("directory")       // full AD directory, all users (key = samaccountname)
	bucketBookings  = []byte("bookings")        // bookings (key = seq)
	bucketTeams     = []byte("teams")           // config_teams (key = teamname)
	bucketRoles     = []byte("roles")           // config_roles (key = id)
	bucketUsers     = []byte("users")           // config_mapadmins + local users (key = username)
	bucketChangelog = []byte("changelog")       // ldap_changelog (key = seq)
	bucketStats     = []byte("stats")           // stats (key = YYYY-MM-DD)
	bucketTracking  = []byte("tracking")        // legacy unique-visitor tracking (no longer written)
	bucketVips      = []byte("vips")            // config_vips (key = seq)
	bucketDepts     = []byte("departments")     // config_department_list (key = seq)
	bucketRobin     = []byte("robinspaces")     // config_robinspaces (key = spacename)
	bucketRobinCfg  = []byte("robinconfig")     // robin org/token/last-sync (key = name)
	bucketMeeting   = []byte("meetingstatus")   // meetingstatus (key = "<map>:<room>")
	bucketWhitelist = []byte("healthwhitelist") // health_whitelist (key = seq)
	bucketLdapSrc   = []byte("ldapsources")     // config_ldap (key = id)
	bucketAudit     = []byte("auditlog")        // auditlog (key = seq)
	bucketMeta      = []byte("meta")            // app meta (wizard state, etc.)
)

var allBuckets = [][]byte{
	bucketSettings, bucketMaps, bucketDesks, bucketLdap, bucketBookings, bucketTeams,
	bucketRoles, bucketUsers, bucketChangelog, bucketStats, bucketTracking, bucketVips,
	bucketDepts, bucketRobin, bucketMeeting, bucketWhitelist, bucketLdapSrc, bucketAudit,
	bucketMeta, bucketDirectory, bucketRobinCfg,
}

type DB struct {
	bolt *bolt.DB
	loc  *time.Location
}

// --- Models ---

// MapInfo mirrors a row of config_maplist.
type MapInfo struct {
	Mapname     string `json:"mapname"`
	DisplayName string `json:"displayname"`
	Itemscale   string `json:"itemscale"`
	Published   string `json:"published"`
	Country     string `json:"country"`
	Flagsize    string `json:"flagsize"`
	Timezone    string `json:"timezone"`
	Address     string `json:"address"`
	MapX        int    `json:"mapX"`
	MapY        int    `json:"mapY"`
}

// Desk mirrors a row of a desks_<map> table, with the map name attached. Desktype
// is the original column-1 value: addesk, localdesk, blocked, hotseat, booking,
// meeting, restroom, food, exit, keycardlock, keylock, printer, service,
// firstaid, floor. (shareddesk is derived at query time, never stored.)
type Desk struct {
	ID         int    `json:"id"`
	Map        string `json:"map"`
	Desktype   string `json:"desktype"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Desknumber string `json:"desknumber"`
	Employee   string `json:"employee"`
	Avatar     string `json:"avatar"`
	Department string `json:"department"`
}

func deskKey(mapName string, id int) []byte {
	return []byte(fmt.Sprintf("%s:%d", mapName, id))
}

// LdapUser mirrors a row of the ldap-mirror table. Userid is the AD account id
// (the old ipphone column) used for avatar filenames.
type LdapUser struct {
	Userid          string `json:"userid"`
	Givenname       string `json:"givenname"`
	Surname         string `json:"surname"`
	Telephonenumber string `json:"telephonenumber"`
	Mail            string `json:"mail"`
	Office          string `json:"physicaldeliveryofficename"`
	Description     string `json:"description"`
	Department      string `json:"department"`
	Mobile          string `json:"mobile"`
}

// DirectoryUser is a single AD account from the full directory snapshot (every
// user, regardless of the office attribute). It powers admin autocomplete and
// name resolution, and is the local source the office-filtered ldapmirror is
// derived from.
type DirectoryUser struct {
	Userid     string `json:"userid"` // samaccountname
	Givenname  string `json:"givenname"`
	Surname    string `json:"surname"`
	Mail       string `json:"mail"`
	Office     string `json:"office"`
	Department string `json:"department"`
	Title      string `json:"title"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
}

// DisplayName returns "Givenname Surname", falling back to the samaccountname.
func (d DirectoryUser) DisplayName() string {
	name := strings.TrimSpace(d.Givenname + " " + d.Surname)
	if name == "" {
		return d.Userid
	}
	return name
}

// Booking mirrors a row of the bookings table.
type Booking struct {
	ID       uint64 `json:"id"`
	Date     string `json:"date"`
	Map      string `json:"map"`
	Desk     string `json:"desk"`
	User     string `json:"user"`
	Fullname string `json:"fullname"`
	Phone    string `json:"phone"`
	Mail     string `json:"mail"`
}

// Team mirrors a row of config_teams.
type Team struct {
	Teamname string `json:"teamname"`
	Members  string `json:"teammembers"`
}

// MembersDisplay renders the pipe-separated member list (the stored format) as a
// comma-separated string for display in the admin UI.
func (t Team) MembersDisplay() string {
	if t.Members == "" {
		return ""
	}
	return strings.ReplaceAll(t.Members, "|", ", ")
}

// Role mirrors a row of config_roles. Perms maps a feature name to a level
// (0=none, 1=read, 2=write).
type Role struct {
	ID       int            `json:"id"`
	Rolename string         `json:"rolename"`
	Perms    map[string]int `json:"perms"`
}

// User merges config_mapadmins (role assignment) with local-login users. For SAML
// or AD-derived admins, IsLocal is false and PassHash/Salt are empty.
type User struct {
	Username  string `json:"username"`
	Role      int    `json:"role"`
	IsLocal   bool   `json:"is_local"`
	PassHash  string `json:"pass_hash,omitempty"`
	Salt      string `json:"salt,omitempty"`
	Fullname  string `json:"fullname,omitempty"`
	Mail      string `json:"mail,omitempty"`
	LastLogin string `json:"last_login,omitempty"`
}

// ChangelogEntry mirrors a row of ldap_changelog.
type ChangelogEntry struct {
	Year     int    `json:"year"`
	Month    int    `json:"month"`
	Day      int    `json:"day"`
	Hour     int    `json:"hour"`
	Minute   int    `json:"minute"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Type     string `json:"type"`
	Oldvalue string `json:"oldvalue"`
	Newvalue string `json:"newvalue"`
}

// StatEntry mirrors a row of the stats table.
type StatEntry struct {
	Date  string `json:"date"`
	Year  int    `json:"year"`
	Month int    `json:"month"`
	Day   int    `json:"day"`
	Count int64  `json:"count"`
}

// VIP mirrors a row of config_vips.
type VIP struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// RobinSpace mirrors a row of config_robinspaces. Spacename is the unique Robin
// location label (also the bucket key); Mapname is the CompanyMaps map the rooms
// are shown on. When Mapname is empty it falls back to Spacename, so multiple
// Robin locations (e.g. goeppingenMain + goeppingenAux) can feed one map.
type RobinSpace struct {
	Spacename string `json:"spacename"`
	Spaceid   int    `json:"spaceid"`
	Mapname   string `json:"mapname,omitempty"`
}

// MapName returns the CompanyMaps map a Robin space feeds, defaulting to the
// space name when no explicit map is configured.
func (s RobinSpace) MapName() string {
	if s.Mapname != "" {
		return s.Mapname
	}
	return s.Spacename
}

// MeetingStatus mirrors a row of the meetingstatus cache table.
type MeetingStatus struct {
	Map          string `json:"map"`
	Room         string `json:"room"`
	Availability string `json:"availability"`
	NowTitle     string `json:"now_title"`
	NowStart     string `json:"now_start"`
	NowEnd       string `json:"now_end"`
	NowTz        string `json:"now_tz"`
	NextTitle    string `json:"next_title"`
	NextStart    string `json:"next_start"`
	NextEnd      string `json:"next_end"`
	NextTz       string `json:"next_tz"`
	Deskid       string `json:"deskid"`
}

// WhitelistEntry mirrors a row of health_whitelist.
type WhitelistEntry struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// LdapSource mirrors a row of config_ldap (an AD sync source). LdapPass is the
// bind password, used for syncing only (never for end-user authentication).
type LdapSource struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Server      string `json:"server"`
	Type        string `json:"type"` // LDAP | LDAPS
	OU          string `json:"OU"`
	LdapUser    string `json:"LdapUser"`
	LdapPass    string `json:"LdapPass"`
	LastSync    string `json:"LastSync"`
}

// AuditEntry mirrors a row of the auditlog table.
type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	User      string `json:"user"`
	Info      string `json:"info"`
}

func openDB(path string) (*DB, error) {
	bdb, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	err = bdb.Update(func(tx *bolt.Tx) error {
		for _, b := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		bdb.Close()
		return nil, fmt.Errorf("creating buckets: %w", err)
	}

	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		loc = time.Local
	}
	db := &DB{bolt: bdb, loc: loc}
	db.migrateRobinConfig()
	return db, nil
}

// migrateRobinConfig moves the Robin org/token/last-sync values out of the
// general settings bucket (where older installs stored them) into the dedicated
// robinconfig bucket, so they no longer clutter the "base variables" list.
func (db *DB) migrateRobinConfig() {
	for _, name := range []string{"robintoken", "robinOrganisation", "robinLastSync"} {
		v, found, _ := getJSON[string](db, bucketSettings, []byte(name))
		if !found {
			continue
		}
		_ = putJSON(db, bucketRobinCfg, []byte(name), v)
		_ = deleteKey(db, bucketSettings, []byte(name))
	}
}

func (db *DB) Close() error { return db.bolt.Close() }

// --- Generic helpers ---

func getJSON[T any](db *DB, bucket, key []byte) (T, bool, error) {
	var out T
	found := false
	err := db.bolt.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucket).Get(key)
		if v == nil {
			return nil
		}
		found = true
		return json.Unmarshal(v, &out)
	})
	return out, found, err
}

func putJSON[T any](db *DB, bucket, key []byte, val T) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(val)
		if err != nil {
			return err
		}
		return tx.Bucket(bucket).Put(key, data)
	})
}

func deleteKey(db *DB, bucket, key []byte) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Delete(key)
	})
}

// listJSON returns all values in a bucket, optionally filtered by a key prefix.
func listJSON[T any](db *DB, bucket []byte, prefix string) ([]T, error) {
	var out []T
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucket).Cursor()
		var k, v []byte
		if prefix != "" {
			p := []byte(prefix)
			for k, v = c.Seek(p); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
				var item T
				if err := json.Unmarshal(v, &item); err == nil {
					out = append(out, item)
				}
				_ = p
			}
		} else {
			for k, v = c.First(); k != nil; k, v = c.Next() {
				var item T
				if err := json.Unmarshal(v, &item); err == nil {
					out = append(out, item)
				}
			}
		}
		return nil
	})
	return out, err
}

func seqKey(seq uint64) []byte {
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, seq)
	return k
}

// --- Settings (config_general) ---

func (db *DB) GetSetting(name string) string {
	v, _, _ := getJSON[string](db, bucketSettings, []byte(name))
	return v
}

func (db *DB) SetSetting(name, value string) error {
	return putJSON(db, bucketSettings, []byte(name), value)
}

// --- Robin configuration (org, token, last-sync) ---

func (db *DB) GetRobinSetting(name string) string {
	v, _, _ := getJSON[string](db, bucketRobinCfg, []byte(name))
	return v
}

func (db *DB) SetRobinSetting(name, value string) error {
	return putJSON(db, bucketRobinCfg, []byte(name), value)
}

func (db *DB) AllSettings() (map[string]string, error) {
	out := map[string]string{}
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSettings).ForEach(func(k, v []byte) error {
			var s string
			if json.Unmarshal(v, &s) == nil {
				out[string(k)] = s
			}
			return nil
		})
	})
	return out, err
}

// --- Maps (config_maplist) ---

func (db *DB) ListMaps() ([]MapInfo, error) {
	maps, err := listJSON[MapInfo](db, bucketMaps, "")
	sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
	return maps, err
}

func (db *DB) GetMap(name string) (MapInfo, bool, error) {
	return getJSON[MapInfo](db, bucketMaps, []byte(name))
}

// MapLocation returns the time.Location for a map, falling back to the database
// default location (Europe/Berlin) when the map has no/invalid timezone.
func (db *DB) MapLocation(name string) *time.Location {
	m, ok, err := db.GetMap(name)
	if err == nil && ok && m.Timezone != "" {
		if loc, lerr := time.LoadLocation(m.Timezone); lerr == nil {
			return loc
		}
	}
	return db.loc
}

// MapToday returns today's date (YYYY-MM-DD) in the map's timezone.
func (db *DB) MapToday(name string) string {
	return time.Now().In(db.MapLocation(name)).Format("2006-01-02")
}

func (db *DB) PutMap(m MapInfo) error { return putJSON(db, bucketMaps, []byte(m.Mapname), m) }

func (db *DB) DeleteMap(name string) error { return deleteKey(db, bucketMaps, []byte(name)) }

// RenameMap renames a map and re-keys everything that references it: the map
// record itself, all of its desks, its cached meeting status, and any bookings.
// It returns an error if the destination name already exists or the source is
// missing. The map image file on disk is handled by the caller. All database
// changes happen atomically in a single transaction.
func (db *DB) RenameMap(oldName, newName string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		mapsB := tx.Bucket(bucketMaps)
		if mapsB.Get([]byte(newName)) != nil {
			return fmt.Errorf("a map named %q already exists", newName)
		}
		raw := mapsB.Get([]byte(oldName))
		if raw == nil {
			return fmt.Errorf("map %q not found", oldName)
		}
		var m MapInfo
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		m.Mapname = newName
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if err := mapsB.Put([]byte(newName), data); err != nil {
			return err
		}
		if err := mapsB.Delete([]byte(oldName)); err != nil {
			return err
		}

		// Re-key desks ("<old>:<id>" -> "<new>:<id>").
		if err := rekeyMapPrefix(tx, bucketDesks, oldName, newName, func(d *Desk) { d.Map = newName }); err != nil {
			return err
		}
		// Re-key cached meeting status ("<old>:<room>" -> "<new>:<room>").
		if err := rekeyMapPrefix(tx, bucketMeeting, oldName, newName, func(s *MeetingStatus) { s.Map = newName }); err != nil {
			return err
		}

		// Update bookings (keyed by seq, value references the map).
		bkB := tx.Bucket(bucketBookings)
		type kv struct{ k, v []byte }
		var updates []kv
		c := bkB.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var b Booking
			if json.Unmarshal(v, &b) != nil || b.Map != oldName {
				continue
			}
			b.Map = newName
			nv, err := json.Marshal(b)
			if err != nil {
				return err
			}
			updates = append(updates, kv{append([]byte(nil), k...), nv})
		}
		for _, u := range updates {
			if err := bkB.Put(u.k, u.v); err != nil {
				return err
			}
		}
		return nil
	})
}

// rekeyMapPrefix moves every "<old>:<suffix>" entry in a bucket to
// "<new>:<suffix>", applying patch to each decoded value before re-storing it.
// Keys are collected first, then mutated, since bolt forbids modifying a bucket
// while iterating its cursor.
func rekeyMapPrefix[T any](tx *bolt.Tx, bucket []byte, oldName, newName string, patch func(*T)) error {
	b := tx.Bucket(bucket)
	prefix := []byte(oldName + ":")
	type move struct{ oldKey, newKey, val []byte }
	var moves []move
	c := b.Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var item T
		if err := json.Unmarshal(v, &item); err != nil {
			return err
		}
		patch(&item)
		nv, err := json.Marshal(item)
		if err != nil {
			return err
		}
		newKey := append([]byte(newName+":"), k[len(prefix):]...)
		moves = append(moves, move{append([]byte(nil), k...), newKey, nv})
	}
	for _, m := range moves {
		if err := b.Put(m.newKey, m.val); err != nil {
			return err
		}
		if err := b.Delete(m.oldKey); err != nil {
			return err
		}
	}
	return nil
}

// --- Desks ---

func (db *DB) ListDesks(mapName string) ([]Desk, error) {
	desks, err := listJSON[Desk](db, bucketDesks, mapName+":")
	return desks, err
}

func (db *DB) GetDesk(mapName string, id int) (Desk, bool, error) {
	return getJSON[Desk](db, bucketDesks, deskKey(mapName, id))
}

func (db *DB) PutDesk(d Desk) error { return putJSON(db, bucketDesks, deskKey(d.Map, d.ID), d) }

func (db *DB) DeleteDesk(mapName string, id int) error {
	return deleteKey(db, bucketDesks, deskKey(mapName, id))
}

// NextDeskID returns the next free desk id for a map.
func (db *DB) NextDeskID(mapName string) (int, error) {
	desks, err := db.ListDesks(mapName)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, d := range desks {
		if d.ID > max {
			max = d.ID
		}
	}
	return max + 1, nil
}

// --- LDAP mirror ---

func (db *DB) ListLdap() ([]LdapUser, error) { return listJSON[LdapUser](db, bucketLdap, "") }

func (db *DB) PutLdap(u LdapUser) error {
	key := u.Userid
	if key == "" {
		key = u.Mail
	}
	return putJSON(db, bucketLdap, []byte(key), u)
}

// ReplaceLdap clears the mirror and stores the given users (used by full sync).
func (db *DB) ReplaceLdap(users []LdapUser) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketLdap); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		b, err := tx.CreateBucket(bucketLdap)
		if err != nil {
			return err
		}
		for _, u := range users {
			key := u.Userid
			if key == "" {
				key = u.Mail
			}
			data, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Directory (full AD snapshot) ---

func (db *DB) ListDirectory() ([]DirectoryUser, error) {
	return listJSON[DirectoryUser](db, bucketDirectory, "")
}

// ReplaceDirectory clears the directory snapshot and stores the given users
// (used by full sync). Keyed by lowercased samaccountname.
func (db *DB) ReplaceDirectory(users []DirectoryUser) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketDirectory); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		b, err := tx.CreateBucket(bucketDirectory)
		if err != nil {
			return err
		}
		for _, u := range users {
			key := strings.ToLower(strings.TrimSpace(u.Userid))
			if key == "" {
				continue
			}
			data, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetDirectoryUser looks up a single account by samaccountname (case-insensitive).
func (db *DB) GetDirectoryUser(sam string) (DirectoryUser, bool, error) {
	return getJSON[DirectoryUser](db, bucketDirectory, []byte(strings.ToLower(strings.TrimSpace(sam))))
}

// SearchDirectory returns up to limit users whose name, samaccountname or mail
// contains the (case-insensitive) query, ordered by display name.
func (db *DB) SearchDirectory(query string, limit int) ([]DirectoryUser, error) {
	all, err := db.ListDirectory()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []DirectoryUser
	for _, u := range all {
		if q == "" {
			out = append(out, u)
			continue
		}
		hay := strings.ToLower(u.DisplayName() + " " + u.Userid + " " + u.Mail)
		if strings.Contains(hay, q) {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName()) < strings.ToLower(out[j].DisplayName())
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// --- Bookings ---

func (db *DB) ListBookings() ([]Booking, error) { return listJSON[Booking](db, bucketBookings, "") }

func (db *DB) AddBooking(b Booking) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketBookings)
		seq, _ := bkt.NextSequence()
		b.ID = seq
		data, err := json.Marshal(b)
		if err != nil {
			return err
		}
		return bkt.Put(seqKey(seq), data)
	})
}

func (db *DB) DeleteBooking(id uint64) error { return deleteKey(db, bucketBookings, seqKey(id)) }

// --- Teams ---

func (db *DB) ListTeams() ([]Team, error) {
	teams, err := listJSON[Team](db, bucketTeams, "")
	sort.Slice(teams, func(i, j int) bool { return teams[i].Teamname < teams[j].Teamname })
	return teams, err
}

func (db *DB) PutTeam(t Team) error { return putJSON(db, bucketTeams, []byte(t.Teamname), t) }

func (db *DB) DeleteTeam(name string) error { return deleteKey(db, bucketTeams, []byte(name)) }

// --- Roles ---

func (db *DB) ListRoles() ([]Role, error) {
	roles, err := listJSON[Role](db, bucketRoles, "")
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	return roles, err
}

func (db *DB) GetRole(id int) (Role, bool, error) {
	return getJSON[Role](db, bucketRoles, []byte(fmt.Sprintf("%d", id)))
}

func (db *DB) PutRole(r Role) error {
	return putJSON(db, bucketRoles, []byte(fmt.Sprintf("%d", r.ID)), r)
}

func (db *DB) DeleteRole(id int) error {
	return deleteKey(db, bucketRoles, []byte(fmt.Sprintf("%d", id)))
}

// --- Users (mapadmins + local) ---

func (db *DB) ListUsers() ([]User, error) { return listJSON[User](db, bucketUsers, "") }

func (db *DB) GetUser(username string) (User, bool, error) {
	return getJSON[User](db, bucketUsers, []byte(username))
}

func (db *DB) PutUser(u User) error { return putJSON(db, bucketUsers, []byte(u.Username), u) }

func (db *DB) DeleteUser(username string) error { return deleteKey(db, bucketUsers, []byte(username)) }

// --- Changelog ---

func (db *DB) AddChangelog(e ChangelogEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketChangelog)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		return bkt.Put(seqKey(seq), data)
	})
}

// ListChangelog returns entries newest-first.
func (db *DB) ListChangelog(limit int) ([]ChangelogEntry, error) {
	var out []ChangelogEntry
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketChangelog).Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var e ChangelogEntry
			if json.Unmarshal(v, &e) == nil {
				out = append(out, e)
			}
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		return nil
	})
	return out, err
}

// --- Stats & tracking ---

func (db *DB) ListStats() ([]StatEntry, error) {
	stats, err := listJSON[StatEntry](db, bucketStats, "")
	sort.Slice(stats, func(i, j int) bool { return stats[i].Date < stats[j].Date })
	return stats, err
}

func (db *DB) PutStat(s StatEntry) error { return putJSON(db, bucketStats, []byte(s.Date), s) }

// AddVisit increments today's visitor count on every call (one per page view),
// matching the legacy PHP stats behaviour.
func (db *DB) AddVisit() error {
	now := time.Now().In(db.loc)
	date := now.Format("2006-01-02")
	return db.bolt.Update(func(tx *bolt.Tx) error {
		sb := tx.Bucket(bucketStats)
		var s StatEntry
		if v := sb.Get([]byte(date)); v != nil {
			_ = json.Unmarshal(v, &s)
		} else {
			s = StatEntry{Date: date, Year: now.Year(), Month: int(now.Month()), Day: now.Day()}
		}
		s.Count++
		data, err := json.Marshal(s)
		if err != nil {
			return err
		}
		return sb.Put([]byte(date), data)
	})
}

// --- VIPs ---

func (db *DB) ListVips() ([]VIP, error) { return listJSON[VIP](db, bucketVips, "") }

func (db *DB) AddVip(v VIP) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketVips)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return bkt.Put(seqKey(seq), data)
	})
}

// AddVipTag adds a tag (parsed job-title text) to a category, ignoring the
// request when an identical tag already exists for that category.
func (db *DB) AddVipTag(typ, title string) error {
	vips, _ := db.ListVips()
	for _, v := range vips {
		if v.Type == typ && strings.EqualFold(v.Title, title) {
			return nil
		}
	}
	return db.AddVip(VIP{Title: title, Type: typ})
}

// DeleteVipTag removes every VIP entry matching the given category and tag.
func (db *DB) DeleteVipTag(typ, title string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketVips)
		var doomed [][]byte
		_ = bkt.ForEach(func(k, val []byte) error {
			var v VIP
			if json.Unmarshal(val, &v) == nil && v.Type == typ && strings.EqualFold(v.Title, title) {
				key := make([]byte, len(k))
				copy(key, k)
				doomed = append(doomed, key)
			}
			return nil
		})
		for _, k := range doomed {
			if err := bkt.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Departments ---

func (db *DB) ListDepartments() ([]string, error) {
	var out []string
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketDepts).ForEach(func(k, v []byte) error {
			var s string
			if json.Unmarshal(v, &s) == nil {
				out = append(out, s)
			}
			return nil
		})
	})
	sort.Strings(out)
	return out, err
}

func (db *DB) AddDepartment(name string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketDepts)
		seq, _ := bkt.NextSequence()
		data, _ := json.Marshal(name)
		return bkt.Put(seqKey(seq), data)
	})
}

// --- Robin spaces ---

func (db *DB) ListRobinSpaces() ([]RobinSpace, error) {
	return listJSON[RobinSpace](db, bucketRobin, "")
}

func (db *DB) PutRobinSpace(s RobinSpace) error {
	return putJSON(db, bucketRobin, []byte(s.Spacename), s)
}

func (db *DB) DeleteRobinSpace(name string) error {
	return deleteKey(db, bucketRobin, []byte(name))
}

// --- Meeting status ---

func meetingKey(mapName, room string) []byte {
	return []byte(mapName + ":" + room)
}

func (db *DB) ListMeetingStatus(mapName string) ([]MeetingStatus, error) {
	if mapName == "" {
		return listJSON[MeetingStatus](db, bucketMeeting, "")
	}
	return listJSON[MeetingStatus](db, bucketMeeting, mapName+":")
}

func (db *DB) PutMeetingStatus(m MeetingStatus) error {
	return putJSON(db, bucketMeeting, meetingKey(m.Map, m.Room), m)
}

// --- Health whitelist ---

func (db *DB) ListWhitelist() ([]WhitelistEntry, error) {
	return listJSON[WhitelistEntry](db, bucketWhitelist, "")
}

func (db *DB) AddWhitelist(e WhitelistEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketWhitelist)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		return bkt.Put(seqKey(seq), data)
	})
}

// --- LDAP sources (config_ldap) ---

func (db *DB) ListLdapSources() ([]LdapSource, error) {
	srcs, err := listJSON[LdapSource](db, bucketLdapSrc, "")
	sort.Slice(srcs, func(i, j int) bool { return srcs[i].ID < srcs[j].ID })
	return srcs, err
}

func (db *DB) PutLdapSource(s LdapSource) error {
	return putJSON(db, bucketLdapSrc, []byte(fmt.Sprintf("%d", s.ID)), s)
}

func (db *DB) DeleteLdapSource(id int) error {
	return deleteKey(db, bucketLdapSrc, []byte(fmt.Sprintf("%d", id)))
}

// --- Audit log ---

func (db *DB) AuditLog(logType, user, message string) error {
	entry := AuditEntry{
		Timestamp: time.Now().In(db.loc).Format("2006-01-02 15:04:05"),
		Type:      logType,
		User:      user,
		Info:      message,
	}
	return db.PutAuditRaw(entry)
}

func (db *DB) PutAuditRaw(entry AuditEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketAudit)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return bkt.Put(seqKey(seq), data)
	})
}

func (db *DB) ListAudit(limit int) ([]AuditEntry, error) {
	var out []AuditEntry
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketAudit).Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var e AuditEntry
			if json.Unmarshal(v, &e) == nil {
				out = append(out, e)
			}
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		return nil
	})
	return out, err
}

// ListAuditPage returns up to limit audit entries (newest first) matching the
// given filters, skipping the first `offset` matches. hasMore reports whether
// further matching entries exist beyond the returned page. Filtering happens in
// the DB scan so the (100k+ on production) audit log is never materialised in
// full – the front-end pages through it via lazy scroll.
func (db *DB) ListAuditPage(offset, limit int, fType, fTime, fUser, fInfo string) ([]AuditEntry, bool, error) {
	if limit <= 0 {
		limit = 100
	}
	fType = strings.ToLower(strings.TrimSpace(fType))
	fTime = strings.ToLower(strings.TrimSpace(fTime))
	fUser = strings.ToLower(strings.TrimSpace(fUser))
	fInfo = strings.ToLower(strings.TrimSpace(fInfo))

	out := make([]AuditEntry, 0, limit)
	hasMore := false
	skipped := 0
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketAudit).Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var e AuditEntry
			if json.Unmarshal(v, &e) != nil {
				continue
			}
			if fType != "" && strings.ToLower(e.Type) != fType {
				continue
			}
			if fTime != "" && !strings.Contains(strings.ToLower(e.Timestamp), fTime) {
				continue
			}
			if fUser != "" && !strings.Contains(strings.ToLower(e.User), fUser) {
				continue
			}
			if fInfo != "" && !strings.Contains(strings.ToLower(e.Info), fInfo) {
				continue
			}
			if skipped < offset {
				skipped++
				continue
			}
			if len(out) == limit {
				hasMore = true
				return nil
			}
			out = append(out, e)
		}
		return nil
	})
	return out, hasMore, err
}

// --- Meta (wizard state, etc.) ---

func (db *DB) GetMeta(key string) string {
	v, _, _ := getJSON[string](db, bucketMeta, []byte(key))
	return v
}

func (db *DB) SetMeta(key, value string) error {
	return putJSON(db, bucketMeta, []byte(key), value)
}

// IsConfigured reports whether the initial setup has been completed.
func (db *DB) IsConfigured() bool {
	return db.GetMeta("setup_done") == "1"
}
