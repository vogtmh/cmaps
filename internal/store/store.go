package store

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
	BucketSettings  = []byte("settings")        // config_general (variable -> value)
	BucketMaps      = []byte("maplist")         // config_maplist (key = mapname)
	BucketDesks     = []byte("desks")           // desks_<map> (key = "<map>:<id>")
	BucketLdap      = []byte("ldapmirror")      // ldap-mirror (key = userid+office, one row per desk placement)
	BucketDirectory = []byte("directory")       // full AD directory, all users (key = samaccountname)
	BucketBookings  = []byte("bookings")        // bookings (key = seq)
	BucketTeams     = []byte("teams")           // config_teams (key = teamname)
	BucketRoles     = []byte("roles")           // config_roles (key = id)
	BucketUsers     = []byte("users")           // config_mapadmins + local users (key = username)
	BucketChangelog = []byte("changelog")       // ldap_changelog (key = seq)
	BucketStats     = []byte("stats")           // stats (key = YYYY-MM-DD)
	BucketTracking  = []byte("tracking")        // legacy unique-visitor tracking (no longer written)
	BucketVips      = []byte("vips")            // config_vips (key = seq)
	BucketDepts     = []byte("departments")     // config_department_list (key = seq)
	BucketRobin     = []byte("robinspaces")     // config_robinspaces (key = spacename)
	BucketRobinCfg  = []byte("robinconfig")     // robin org/token/last-sync (key = name)
	BucketRobinDesk = []byte("robindeskstatus") // live Robin seat occupancy (key = "<map>:<desknumber>")
	BucketMeeting   = []byte("meetingstatus")   // meetingstatus (key = "<map>:<room>")
	BucketWhitelist = []byte("healthwhitelist") // health_whitelist (key = seq)
	BucketLdapSrc   = []byte("ldapsources")     // config_ldap (key = id)
	BucketAudit     = []byte("auditlog")        // auditlog (key = seq)
	BucketMeta      = []byte("meta")            // app meta (wizard state, etc.)
	BucketGeoCfg    = []byte("geoconfig")       // geocoding (geoapify) api key / settings (key = name)
	BucketItemTypes = []byte("itemtypes")       // admin-defined custom palette item types (key = id)
	BucketEntraLdap = []byte("entraidmirror")   // EntraID (Graph) office-filtered mirror, same shape as ldapmirror
	BucketEntraCfg  = []byte("entraidconfig")   // EntraID app-registration credentials / last-sync (key = name)
	BucketEntraSrc  = []byte("entraidsources")  // EntraID app registrations, one per connection (key = id)
	BucketSrcMirror = []byte("sourcemirror")    // per-source desk placements (key = "ldap:<id>"/"entra:<id>" -> JSON []LdapUser)
	BucketSrcDir    = []byte("sourcedir")       // per-source full directory snapshot (key = "ldap:<id>" -> JSON []DirectoryUser)
)

var allBuckets = [][]byte{
	BucketSettings, BucketMaps, BucketDesks, BucketLdap, BucketBookings, BucketTeams,
	BucketRoles, BucketUsers, BucketChangelog, BucketStats, BucketTracking, BucketVips,
	BucketDepts, BucketRobin, BucketMeeting, BucketWhitelist, BucketLdapSrc, BucketAudit,
	BucketMeta, BucketDirectory, BucketRobinCfg, BucketRobinDesk, BucketGeoCfg, BucketItemTypes,
	BucketEntraLdap, BucketEntraCfg, BucketEntraSrc, BucketSrcMirror, BucketSrcDir,
}

type DB struct {
	bolt *bolt.DB
	loc  *time.Location
}

// --- Models ---

// MapInfo mirrors a row of config_maplist.
type MapInfo struct {
	Mapname     string  `json:"mapname"`
	DisplayName string  `json:"displayname"`
	Itemscale   string  `json:"itemscale"`
	Published   string  `json:"published"`
	Country     string  `json:"country"`
	Timezone    string  `json:"timezone"`
	Address     string  `json:"address"`
	MapX        int     `json:"mapX"`
	MapY        int     `json:"mapY"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
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

func DeskKey(mapName string, id int) []byte {
	return []byte(fmt.Sprintf("%s:%d", mapName, id))
}

// LdapUser mirrors a row of the ldap-mirror table. Userid is the AD account id
// (the old ipphone column) used for avatar filenames.
type LdapUser struct {
	Userid          string `json:"userid"`
	// Samaccountname is the raw AD account name, kept independently of Userid so
	// the identifier can be switched between samaccountname and mail (and back).
	Samaccountname  string `json:"samaccountname,omitempty"`
	Givenname       string `json:"givenname"`
	Surname         string `json:"surname"`
	Telephonenumber string `json:"telephonenumber"`
	Mail            string `json:"mail"`
	Office          string `json:"physicaldeliveryofficename"`
	Description     string `json:"description"`
	Department      string `json:"department"`
	Mobile          string `json:"mobile"`
	// Aliases are additional SMTP addresses (from AD proxyAddresses) that also
	// resolve to this person, e.g. a legacy "spaeth@" before "first.last@".
	Aliases []string `json:"aliases,omitempty"`
	// HasAvatar is set during the hourly LDAP sync (and on manual upload/delete)
	// to whether avatarcache/<userid>.jpg exists, so the client can point at a
	// single shared placeholder for users without one instead of requesting a
	// unique missing image per person.
	HasAvatar bool `json:"hasavatar"`
}

// DirectoryUser is a single AD account from the full directory snapshot (every
// user, regardless of the office attribute). It powers admin autocomplete and
// name resolution, and is the local source the office-filtered ldapmirror is
// derived from.
type DirectoryUser struct {
	Userid string `json:"userid"` // active identifier (samaccountname or mail-based)
	// Samaccountname is the raw AD account name, retained regardless of the
	// configured identifier mode so migrations can round-trip.
	Samaccountname string `json:"samaccountname,omitempty"`
	Givenname      string `json:"givenname"`
	Surname    string `json:"surname"`
	Mail       string `json:"mail"`
	Office     string `json:"office"`
	Department string `json:"department"`
	Title      string `json:"title"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
	// Aliases are additional SMTP addresses (AD proxyAddresses) besides Mail.
	Aliases []string `json:"aliases,omitempty"`
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

// RobinDeskStatus is a single live Robin seat reservation that is active right
// now, resolved to a CompanyMaps desk. It is cached by the Robin scheduler and
// read by the desk overlay. It is intentionally separate from the native
// booking feature and the meeting cache.
type RobinDeskStatus struct {
	Map        string `json:"map"`
	Desknumber string `json:"desknumber"`
	Name       string `json:"name"`
	Userid     string `json:"userid"`
	Mail       string `json:"mail"`
	Phone      string `json:"phone"`
	Title      string `json:"title"`
	Mobile     string `json:"mobile"`
	Type       string `json:"type"`
	End        string `json:"end"`
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
	// Disabled excludes the source from the combined AD sync. Stored as
	// omitempty so existing sources (which predate this field) default to
	// enabled, matching "enabled unless the user deactivates it".
	Disabled bool `json:"disabled,omitempty"`
	// Demo marks the built-in demo source. Its sync never connects to a real
	// directory (see fetchSourceDirectory): it regenerates the bundled demo
	// employees so the sync can never fail. It is otherwise a normal LDAP-type
	// source in the priority list, filling the demo desks.
	Demo bool `json:"demo,omitempty"`
}

// EntraSource is one Microsoft Entra ID app registration used as a directory
// sync source (Microsoft Graph). It mirrors LdapSource's role for AD: several
// connections can be configured and each is synced into the shared EntraID
// mirror. Secrets/certificate material are used for syncing only.
type EntraSource struct {
	ID           int    `json:"id"`
	Description  string `json:"description"`
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	AuthMethod   string `json:"auth_method"` // secret | certificate
	ClientSecret string `json:"client_secret,omitempty"`
	CertPEM      string `json:"cert_pem,omitempty"`
	KeyPEM       string `json:"key_pem,omitempty"`
	LastSync     string `json:"last_sync"`
	// Disabled excludes the source from the combined EntraID sync. omitempty so
	// pre-existing/migrated sources default to enabled.
	Disabled bool `json:"disabled,omitempty"`
}

// AuditEntry mirrors a row of the auditlog table.
type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	User      string `json:"user"`
	Info      string `json:"info"`
}

func Open(path string) (*DB, error) {
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
	db.removeObsoleteSettings()
	db.pinLegacyIdentifier()
	return db, nil
}

// migrateRobinConfig moves the Robin org/token/last-sync values out of the
// general settings bucket (where older installs stored them) into the dedicated
// robinconfig bucket, so they no longer clutter the "base variables" list.
func (db *DB) migrateRobinConfig() {
	for _, name := range []string{"robintoken", "robinOrganisation", "robinLastSync"} {
		v, found, _ := GetJSON[string](db, BucketSettings, []byte(name))
		if !found {
			continue
		}
		_ = PutJSON(db, BucketRobinCfg, []byte(name), v)
		_ = DeleteKey(db, BucketSettings, []byte(name))
	}
}

// removeObsoleteSettings deletes general settings that are no longer used by the
// application so they stop appearing in the admin "base variables" list.
func (db *DB) removeObsoleteSettings() {
	for _, name := range []string{"teamsChannel", "avatarType"} {
		_ = DeleteKey(db, BucketSettings, []byte(name))
	}
}

// pinLegacyIdentifier protects installs created before the identifier default
// switched to "mail". Such installs have no explicit "identifier" setting and
// rely on the old samaccountname default for their avatar filenames, map-admin
// records, bookings and audit log. The first time we start up on an
// already-configured install with no setting, pin it to samaccountname so the
// new mail default only ever affects genuinely fresh installs.
func (db *DB) pinLegacyIdentifier() {
	if db.GetMeta("setup_done") != "1" {
		return
	}
	if v, found, _ := GetJSON[string](db, BucketSettings, []byte("identifier")); found && v != "" {
		return
	}
	_ = PutJSON(db, BucketSettings, []byte("identifier"), "samaccountname")
}

func (db *DB) Close() error { return db.bolt.Close() }

// --- Generic helpers ---

func GetJSON[T any](db *DB, bucket, key []byte) (T, bool, error) {
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

func PutJSON[T any](db *DB, bucket, key []byte, val T) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(val)
		if err != nil {
			return err
		}
		return tx.Bucket(bucket).Put(key, data)
	})
}

func DeleteKey(db *DB, bucket, key []byte) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Delete(key)
	})
}

// ListJSON returns all values in a bucket, optionally filtered by a key prefix.
func ListJSON[T any](db *DB, bucket []byte, prefix string) ([]T, error) {
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

func SeqKey(seq uint64) []byte {
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, seq)
	return k
}

// --- Settings (config_general) ---

func (db *DB) GetSetting(name string) string {
	v, _, _ := GetJSON[string](db, BucketSettings, []byte(name))
	return v
}

func (db *DB) SetSetting(name, value string) error {
	return PutJSON(db, BucketSettings, []byte(name), value)
}

// EnsureSetting creates the setting with the given default only if it does not
// already exist, leaving any existing value untouched. Used to surface newer
// optional settings in the admin panel on upgraded installations.
func (db *DB) EnsureSetting(name, def string) error {
	if _, ok, _ := GetJSON[string](db, BucketSettings, []byte(name)); ok {
		return nil
	}
	return db.SetSetting(name, def)
}

// --- Robin configuration (org, token, last-sync) ---

func (db *DB) GetRobinSetting(name string) string {
	v, _, _ := GetJSON[string](db, BucketRobinCfg, []byte(name))
	return v
}

func (db *DB) SetRobinSetting(name, value string) error {
	return PutJSON(db, BucketRobinCfg, []byte(name), value)
}

// --- Geocoding configuration (geoapify api key, kept out of the visible
// config_general table since it is a secret) ---

func (db *DB) GetGeoSetting(name string) string {
	v, _, _ := GetJSON[string](db, BucketGeoCfg, []byte(name))
	return v
}

func (db *DB) SetGeoSetting(name, value string) error {
	return PutJSON(db, BucketGeoCfg, []byte(name), value)
}

// geoUsage tracks how many Geoapify API requests this server has made in a
// given calendar month, so the admin panel can show a running estimate of
// monthly consumption (Geoapify has no public quota-query endpoint).
type geoUsage struct {
	Month string `json:"month"` // "2006-01"
	Count int    `json:"count"`
}

// GetGeoUsage returns the request count for the current calendar month. If the
// stored counter belongs to an earlier month it reports zero (a new month has
// begun and the estimate resets).
func (db *DB) GetGeoUsage() (month string, count int) {
	now := time.Now().Format("2006-01")
	u, _, _ := GetJSON[geoUsage](db, BucketGeoCfg, []byte("usage"))
	if u.Month != now {
		return now, 0
	}
	return u.Month, u.Count
}

// IncrGeoUsage adds n to the current month's request counter, resetting it when
// the month rolls over, and returns the updated month and total.
func (db *DB) IncrGeoUsage(n int) (month string, count int, err error) {
	now := time.Now().Format("2006-01")
	u, _, _ := GetJSON[geoUsage](db, BucketGeoCfg, []byte("usage"))
	if u.Month != now {
		u.Month = now
		u.Count = 0
	}
	u.Count += n
	if err := PutJSON(db, BucketGeoCfg, []byte("usage"), u); err != nil {
		return u.Month, u.Count, err
	}
	return u.Month, u.Count, nil
}

func (db *DB) AllSettings() (map[string]string, error) {
	out := map[string]string{}
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return tx.Bucket(BucketSettings).ForEach(func(k, v []byte) error {
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
	maps, err := ListJSON[MapInfo](db, BucketMaps, "")
	sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
	return maps, err
}

func (db *DB) GetMap(name string) (MapInfo, bool, error) {
	return GetJSON[MapInfo](db, BucketMaps, []byte(name))
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

func (db *DB) PutMap(m MapInfo) error { return PutJSON(db, BucketMaps, []byte(m.Mapname), m) }

func (db *DB) DeleteMap(name string) error { return DeleteKey(db, BucketMaps, []byte(name)) }

// RenameMap renames a map and re-keys everything that references it: the map
// record itself, all of its desks, its cached meeting status, and any bookings.
// It returns an error if the destination name already exists or the source is
// missing. The map image file on disk is handled by the caller. All database
// changes happen atomically in a single transaction.
func (db *DB) RenameMap(oldName, newName string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		mapsB := tx.Bucket(BucketMaps)
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
		if err := rekeyMapPrefix(tx, BucketDesks, oldName, newName, func(d *Desk) { d.Map = newName }); err != nil {
			return err
		}
		// Re-key cached meeting status ("<old>:<room>" -> "<new>:<room>").
		if err := rekeyMapPrefix(tx, BucketMeeting, oldName, newName, func(s *MeetingStatus) { s.Map = newName }); err != nil {
			return err
		}

		// Update bookings (keyed by seq, value references the map).
		bkB := tx.Bucket(BucketBookings)
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
	desks, err := ListJSON[Desk](db, BucketDesks, mapName+":")
	return desks, err
}

// ListAllDesks returns every desk/item record across all maps. Used for the
// dashboard overview counts.
func (db *DB) ListAllDesks() ([]Desk, error) {
	return ListJSON[Desk](db, BucketDesks, "")
}

func (db *DB) GetDesk(mapName string, id int) (Desk, bool, error) {
	return GetJSON[Desk](db, BucketDesks, DeskKey(mapName, id))
}

func (db *DB) PutDesk(d Desk) error { return PutJSON(db, BucketDesks, DeskKey(d.Map, d.ID), d) }

func (db *DB) DeleteDesk(mapName string, id int) error {
	return DeleteKey(db, BucketDesks, DeskKey(mapName, id))
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

// --- Custom item types ---

// CustomItemType is an admin-defined palette item (marker) that editors can drag
// onto a map. It renders with the configured colour/icon/size and is stored on
// desks with desktype "custom_<ID>".
type CustomItemType struct {
	ID          string `json:"id"`          // url-safe slug, also the icon filename stem
	Label       string `json:"label"`       // shown in the palette + as the marker's name
	Description string `json:"description"` // palette tile subtitle / tooltip
	Color       string `json:"color"`       // CSS colour for the marker background
	Icon        string `json:"icon"`        // uploaded icon filename (served from /itemicons/), or ""
	Size        string `json:"size"`        // "small" | "medium" | "large"
}

// Halfsize maps the named size onto the same half-box pixel scale used by the
// built-in item types (see editItemHalfsize in admin.js / updateDesks in user.js).
func (t CustomItemType) Halfsize() int {
	switch t.Size {
	case "small":
		return 18
	case "large":
		return 40
	default:
		return 25
	}
}

func (db *DB) ListItemTypes() ([]CustomItemType, error) {
	return ListJSON[CustomItemType](db, BucketItemTypes, "")
}

func (db *DB) GetItemType(id string) (CustomItemType, bool, error) {
	return GetJSON[CustomItemType](db, BucketItemTypes, []byte(id))
}

func (db *DB) PutItemType(t CustomItemType) error {
	return PutJSON(db, BucketItemTypes, []byte(t.ID), t)
}

func (db *DB) DeleteItemType(id string) error {
	return DeleteKey(db, BucketItemTypes, []byte(id))
}

// --- LDAP mirror ---

func (db *DB) ListLdap() ([]LdapUser, error) { return ListJSON[LdapUser](db, BucketLdap, "") }

// ldapKey builds the mirror bucket key for a placement. A user may hold several
// desks (office "A|B" split into separate placements by deriveMirrorUsers), so
// the key combines userid and office to keep every placement distinct. Without
// the office component all but the last placement of a multi-desk user would
// overwrite each other, dropping the person from every desk except one.
func ldapKey(u LdapUser) []byte {
	id := u.Userid
	if id == "" {
		id = u.Mail
	}
	return []byte(id + "\x00" + u.Office)
}

func (db *DB) PutLdap(u LdapUser) error {
	return PutJSON(db, BucketLdap, ldapKey(u), u)
}

// ReplaceLdap clears the mirror and stores the given users (used by full sync).
func (db *DB) ReplaceLdap(users []LdapUser) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(BucketLdap); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		b, err := tx.CreateBucket(BucketLdap)
		if err != nil {
			return err
		}
		for _, u := range users {
			data, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := b.Put(ldapKey(u), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetLdapAvatar updates the stored HasAvatar flag for a mirrored user. Called
// after a manual avatar upload/delete so the change is reflected immediately
// without waiting for the next hourly sync. A user may have several placements
// (one per desk), so every entry with a matching userid is updated. A no-op if
// the user is not in the mirror.
func (db *DB) SetLdapAvatar(userid string, has bool) error {
	if userid == "" {
		return nil
	}
	return db.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(BucketLdap)
		if b == nil {
			return nil
		}
		// Collect the keys to update first: mutating the bucket while iterating
		// with a cursor can invalidate it.
		var keys [][]byte
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u LdapUser
			if err := json.Unmarshal(v, &u); err != nil {
				return err
			}
			if u.Userid == userid && u.HasAvatar != has {
				key := make([]byte, len(k))
				copy(key, k)
				keys = append(keys, key)
			}
		}
		for _, k := range keys {
			var u LdapUser
			if err := json.Unmarshal(b.Get(k), &u); err != nil {
				return err
			}
			u.HasAvatar = has
			nd, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := b.Put(k, nd); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Per-source mirrors (combine-on-write) ---
//
// Each sync source (LDAP or EntraID) stores its own derived desk placements
// (and, for LDAP, its full directory snapshot) under its own key. The shared
// combined caches (BucketLdap/BucketEntraLdap/BucketDirectory) are then rebuilt
// by unioning the enabled sources, so a single-source sync never wipes the
// others. Keys are "ldap:<id>" and "entra:<id>".

func srcKey(kind string, id int) []byte {
	return []byte(fmt.Sprintf("%s:%d", kind, id))
}

// PutSourceMirror stores one source's derived desk placements.
func (db *DB) PutSourceMirror(kind string, id int, users []LdapUser) error {
	return PutJSON(db, BucketSrcMirror, srcKey(kind, id), users)
}

// GetSourceMirror returns one source's derived desk placements (nil if unset).
func (db *DB) GetSourceMirror(kind string, id int) ([]LdapUser, error) {
	users, _, err := GetJSON[[]LdapUser](db, BucketSrcMirror, srcKey(kind, id))
	return users, err
}

// DeleteSourceMirror removes one source's derived desk placements.
func (db *DB) DeleteSourceMirror(kind string, id int) error {
	return DeleteKey(db, BucketSrcMirror, srcKey(kind, id))
}

// PutSourceDir stores one LDAP source's full directory snapshot.
func (db *DB) PutSourceDir(id int, users []DirectoryUser) error {
	return PutJSON(db, BucketSrcDir, srcKey("ldap", id), users)
}

// GetSourceDir returns one LDAP source's full directory snapshot (nil if unset).
func (db *DB) GetSourceDir(id int) ([]DirectoryUser, error) {
	users, _, err := GetJSON[[]DirectoryUser](db, BucketSrcDir, srcKey("ldap", id))
	return users, err
}

// DeleteSourceDir removes one LDAP source's full directory snapshot.
func (db *DB) DeleteSourceDir(id int) error {
	return DeleteKey(db, BucketSrcDir, srcKey("ldap", id))
}

// --- EntraID mirror (Microsoft Graph, same shape as the LDAP mirror) ---
//
// The EntraID directory sync stores its office-filtered desk placements in a
// separate bucket so it never overwrites the LDAP mirror and the two can be
// compared side by side. It reuses LdapUser and ldapKey for an apples-to-apples
// comparison with the LDAP data.

func (db *DB) ListEntraLdap() ([]LdapUser, error) {
	return ListJSON[LdapUser](db, BucketEntraLdap, "")
}

// ReplaceEntraLdap clears the EntraID mirror and stores the given users (full sync).
func (db *DB) ReplaceEntraLdap(users []LdapUser) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(BucketEntraLdap); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		b, err := tx.CreateBucket(BucketEntraLdap)
		if err != nil {
			return err
		}
		for _, u := range users {
			data, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := b.Put(ldapKey(u), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- EntraID configuration (tenant, client id, auth method + secret/cert) ---

func (db *DB) GetEntraSetting(name string) string {
	v, _, _ := GetJSON[string](db, BucketEntraCfg, []byte(name))
	return v
}

func (db *DB) SetEntraSetting(name, value string) error {
	return PutJSON(db, BucketEntraCfg, []byte(name), value)
}

// --- EntraID sources (one row per app registration / connection) ---

func (db *DB) ListEntraSources() ([]EntraSource, error) {
	srcs, err := ListJSON[EntraSource](db, BucketEntraSrc, "")
	sort.Slice(srcs, func(i, j int) bool { return srcs[i].ID < srcs[j].ID })
	return srcs, err
}

func (db *DB) PutEntraSource(s EntraSource) error {
	return PutJSON(db, BucketEntraSrc, []byte(fmt.Sprintf("%d", s.ID)), s)
}

func (db *DB) DeleteEntraSource(id int) error {
	return DeleteKey(db, BucketEntraSrc, []byte(fmt.Sprintf("%d", id)))
}

// --- Directory (full AD snapshot) ---

func (db *DB) ListDirectory() ([]DirectoryUser, error) {
	return ListJSON[DirectoryUser](db, BucketDirectory, "")
}

// ReplaceDirectory clears the directory snapshot and stores the given users
// (used by full sync). Keyed by lowercased samaccountname.
func (db *DB) ReplaceDirectory(users []DirectoryUser) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(BucketDirectory); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		b, err := tx.CreateBucket(BucketDirectory)
		if err != nil {
			return err
		}
		for _, u := range users {
			key := strings.ToLower(strings.TrimSpace(u.Userid))
			if key == "" {
				continue
			}
			// Defensive sanity check: mail addresses are always stored strictly
			// lowercased regardless of which sync path produced this record.
			u.Mail = NormalizeMail(u.Mail)
			u.Aliases = NormalizeMails(u.Aliases)
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
	return GetJSON[DirectoryUser](db, BucketDirectory, []byte(strings.ToLower(strings.TrimSpace(sam))))
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

func (db *DB) ListBookings() ([]Booking, error) { return ListJSON[Booking](db, BucketBookings, "") }

func (db *DB) AddBooking(b Booking) error {
	// Defensive sanity check: mail addresses are always stored strictly lowercased.
	b.Mail = NormalizeMail(b.Mail)
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(BucketBookings)
		seq, _ := bkt.NextSequence()
		b.ID = seq
		data, err := json.Marshal(b)
		if err != nil {
			return err
		}
		return bkt.Put(SeqKey(seq), data)
	})
}

func (db *DB) DeleteBooking(id uint64) error { return DeleteKey(db, BucketBookings, SeqKey(id)) }

// --- Teams ---

func (db *DB) ListTeams() ([]Team, error) {
	teams, err := ListJSON[Team](db, BucketTeams, "")
	sort.Slice(teams, func(i, j int) bool { return teams[i].Teamname < teams[j].Teamname })
	return teams, err
}

func (db *DB) PutTeam(t Team) error { return PutJSON(db, BucketTeams, []byte(t.Teamname), t) }

func (db *DB) DeleteTeam(name string) error { return DeleteKey(db, BucketTeams, []byte(name)) }

// --- Roles ---

func (db *DB) ListRoles() ([]Role, error) {
	roles, err := ListJSON[Role](db, BucketRoles, "")
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	return roles, err
}

func (db *DB) GetRole(id int) (Role, bool, error) {
	return GetJSON[Role](db, BucketRoles, []byte(fmt.Sprintf("%d", id)))
}

func (db *DB) PutRole(r Role) error {
	return PutJSON(db, BucketRoles, []byte(fmt.Sprintf("%d", r.ID)), r)
}

func (db *DB) DeleteRole(id int) error {
	return DeleteKey(db, BucketRoles, []byte(fmt.Sprintf("%d", id)))
}

// --- Users (mapadmins + local) ---

func (db *DB) ListUsers() ([]User, error) { return ListJSON[User](db, BucketUsers, "") }

func (db *DB) GetUser(username string) (User, bool, error) {
	return GetJSON[User](db, BucketUsers, []byte(username))
}

func (db *DB) PutUser(u User) error {
	// Defensive sanity check: mail addresses are always stored strictly lowercased.
	u.Mail = NormalizeMail(u.Mail)
	return PutJSON(db, BucketUsers, []byte(u.Username), u)
}

func (db *DB) DeleteUser(username string) error { return DeleteKey(db, BucketUsers, []byte(username)) }

// --- Changelog ---

func (db *DB) AddChangelog(e ChangelogEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(BucketChangelog)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		return bkt.Put(SeqKey(seq), data)
	})
}

// ListChangelog returns entries newest-first.
func (db *DB) ListChangelog(limit int) ([]ChangelogEntry, error) {
	var out []ChangelogEntry
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(BucketChangelog).Cursor()
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
	stats, err := ListJSON[StatEntry](db, BucketStats, "")
	sort.Slice(stats, func(i, j int) bool { return stats[i].Date < stats[j].Date })
	return stats, err
}

func (db *DB) PutStat(s StatEntry) error { return PutJSON(db, BucketStats, []byte(s.Date), s) }

// AddVisit increments today's visitor count on every call (one per page view),
// matching the legacy PHP stats behaviour.
func (db *DB) AddVisit() error {
	now := time.Now().In(db.loc)
	date := now.Format("2006-01-02")
	return db.bolt.Update(func(tx *bolt.Tx) error {
		sb := tx.Bucket(BucketStats)
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

func (db *DB) ListVips() ([]VIP, error) { return ListJSON[VIP](db, BucketVips, "") }

func (db *DB) AddVip(v VIP) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(BucketVips)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return bkt.Put(SeqKey(seq), data)
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
		bkt := tx.Bucket(BucketVips)
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
		return tx.Bucket(BucketDepts).ForEach(func(k, v []byte) error {
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
		bkt := tx.Bucket(BucketDepts)
		seq, _ := bkt.NextSequence()
		data, _ := json.Marshal(name)
		return bkt.Put(SeqKey(seq), data)
	})
}

// --- Robin spaces ---

func (db *DB) ListRobinSpaces() ([]RobinSpace, error) {
	return ListJSON[RobinSpace](db, BucketRobin, "")
}

func (db *DB) PutRobinSpace(s RobinSpace) error {
	return PutJSON(db, BucketRobin, []byte(s.Spacename), s)
}

func (db *DB) DeleteRobinSpace(name string) error {
	return DeleteKey(db, BucketRobin, []byte(name))
}

// --- Robin desk occupancy cache (robindeskstatus) ---

func robinDeskKey(mapName, desknumber string) []byte {
	return []byte(mapName + ":" + desknumber)
}

// ListRobinDeskStatus returns the cached live Robin occupancy for a map (or all
// maps when mapName is empty).
func (db *DB) ListRobinDeskStatus(mapName string) ([]RobinDeskStatus, error) {
	if mapName == "" {
		return ListJSON[RobinDeskStatus](db, BucketRobinDesk, "")
	}
	return ListJSON[RobinDeskStatus](db, BucketRobinDesk, mapName+":")
}

// ReplaceRobinDeskStatus atomically clears the occupancy cache and writes the
// supplied set, so a poll fully replaces the previous snapshot.
func (db *DB) ReplaceRobinDeskStatus(all []RobinDeskStatus) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(BucketRobinDesk); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		bkt, err := tx.CreateBucketIfNotExists(BucketRobinDesk)
		if err != nil {
			return err
		}
		for _, s := range all {
			data, err := json.Marshal(s)
			if err != nil {
				return err
			}
			if err := bkt.Put(robinDeskKey(s.Map, s.Desknumber), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Meeting status ---

func meetingKey(mapName, room string) []byte {
	return []byte(mapName + ":" + room)
}

func (db *DB) ListMeetingStatus(mapName string) ([]MeetingStatus, error) {
	if mapName == "" {
		return ListJSON[MeetingStatus](db, BucketMeeting, "")
	}
	return ListJSON[MeetingStatus](db, BucketMeeting, mapName+":")
}

func (db *DB) PutMeetingStatus(m MeetingStatus) error {
	return PutJSON(db, BucketMeeting, meetingKey(m.Map, m.Room), m)
}

// --- Health whitelist ---

func (db *DB) ListWhitelist() ([]WhitelistEntry, error) {
	return ListJSON[WhitelistEntry](db, BucketWhitelist, "")
}

func (db *DB) AddWhitelist(e WhitelistEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(BucketWhitelist)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		return bkt.Put(SeqKey(seq), data)
	})
}

// DeleteWhitelist removes every whitelist entry matching the given type and
// text (there may be duplicates). Keys are collected first and deleted after
// the cursor scan, since bbolt forbids mutating a bucket during iteration.
func (db *DB) DeleteWhitelist(e WhitelistEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(BucketWhitelist)
		var toDelete [][]byte
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry WhitelistEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}
			if entry.Type == e.Type && entry.Text == e.Text {
				key := make([]byte, len(k))
				copy(key, k)
				toDelete = append(toDelete, key)
			}
		}
		for _, k := range toDelete {
			if err := bkt.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- LDAP sources (config_ldap) ---

func (db *DB) ListLdapSources() ([]LdapSource, error) {
	srcs, err := ListJSON[LdapSource](db, BucketLdapSrc, "")
	sort.Slice(srcs, func(i, j int) bool { return srcs[i].ID < srcs[j].ID })
	return srcs, err
}

func (db *DB) PutLdapSource(s LdapSource) error {
	return PutJSON(db, BucketLdapSrc, []byte(fmt.Sprintf("%d", s.ID)), s)
}

func (db *DB) DeleteLdapSource(id int) error {
	return DeleteKey(db, BucketLdapSrc, []byte(fmt.Sprintf("%d", id)))
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
		bkt := tx.Bucket(BucketAudit)
		seq, _ := bkt.NextSequence()
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return bkt.Put(SeqKey(seq), data)
	})
}

// ReplaceAudit wipes the audit bucket and re-fills it with the given entries in
// order (so entries[0] is treated as the oldest and gets the lowest sequence,
// keeping the reverse-cursor listing newest-first). Deleting and recreating the
// bucket resets its sequence counter, so subsequently appended live events stay
// above the imported history. Used by the superadmin one-time audit re-import.
func (db *DB) ReplaceAudit(entries []AuditEntry) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(BucketAudit); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		bkt, err := tx.CreateBucket(BucketAudit)
		if err != nil {
			return err
		}
		for _, e := range entries {
			seq, _ := bkt.NextSequence()
			data, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if err := bkt.Put(SeqKey(seq), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *DB) ListAudit(limit int) ([]AuditEntry, error) {
	var out []AuditEntry
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(BucketAudit).Cursor()
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
		c := tx.Bucket(BucketAudit).Cursor()
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
	v, _, _ := GetJSON[string](db, BucketMeta, []byte(key))
	return v
}

func (db *DB) SetMeta(key, value string) error {
	return PutJSON(db, BucketMeta, []byte(key), value)
}

// IsConfigured reports whether the initial setup has been completed.
func (db *DB) IsConfigured() bool {
	return db.GetMeta("setup_done") == "1"
}

// NormalizeMail canonicalises an e-mail address for storage: trimmed and
// strictly lowercased. Mail addresses are case-insensitive for our purposes
// (matching, identifiers, display), so every directory source runs its mail
// values through this before persisting.
func NormalizeMail(mail string) string {
	return strings.ToLower(strings.TrimSpace(mail))
}

// NormalizeMails canonicalises a slice of mail addresses (e.g. AD
// proxyAddresses aliases), dropping any that are empty after trimming.
func NormalizeMails(mails []string) []string {
	out := make([]string, 0, len(mails))
	for _, m := range mails {
		if n := NormalizeMail(m); n != "" {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SourceRule is one entry of the stored directory-source priority list. Ref
// identifies the source: "ldap:<id>", "entra:<id>" or "robin". Slice order is
// the priority (index 0 = highest). The two flags are stored inverted/
// omitempty so entries added automatically (or migrated from older installs)
// default to "assigns desks, deduplicates people".
type SourceRule struct {
	Ref string `json:"ref"`
	// KeepDuplicates lets this (lower priority) source place a person again even
	// when a higher-priority source already placed them on the same map. It never
	// lets it overwrite a desk a higher-priority source already owns.
	KeepDuplicates bool `json:"keepDuplicates,omitempty"`
	// NoAssign excludes the source from desk assignment while leaving its sync
	// intact. Stored inverted so existing rules assign by default.
	NoAssign bool `json:"noAssign,omitempty"`
}

var metaSourceOrder = []byte("directorySourceOrder")

// GetSourceOrder returns the stored ordered priority list (nil if never set).
func (db *DB) GetSourceOrder() []SourceRule {
	rules, _, _ := GetJSON[[]SourceRule](db, BucketMeta, metaSourceOrder)
	return rules
}

// SetSourceOrder persists the ordered priority list.
func (db *DB) SetSourceOrder(rules []SourceRule) error {
	return PutJSON(db, BucketMeta, metaSourceOrder, rules)
}

// Bolt exposes the underlying bolt handle. Transitional: only the backup
// export/import and the identifier migration engine still need raw
// transactions; both are scheduled to move into this package.
func (db *DB) Bolt() *bolt.DB { return db.bolt }

// Location returns the display timezone used for admin-facing timestamps.
func (db *DB) Location() *time.Location { return db.loc }
