package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// floorButton is a floor selector entry in the header.
type floorButton struct {
	ID       int
	Employee string
}

// mapLink is one entry in the location dropdown: Name is the map identifier
// (used in the ?map= URL), Label is the user-facing display name.
type mapLink struct {
	Name  string
	Label string
}

// indexData holds everything the index.html template needs. It reproduces the
// server-side state the legacy index.php computed before handing off to the
// JavaScript front-end.
type indexData struct {
	AppTitle          string
	TargetScreenWidth int
	HalfWidth         int
	Map               string
	MapTitle          string
	MapDefault        string
	MapList           []string
	OtherMaps         []mapLink
	IsOverview        bool
	Itemscale         string
	Autozoom          int
	Zoom              int
	ZoomIn            int
	ZoomOut           int
	ZoomScaled        string // zoom/100
	ContentScale      string // zoom/100*autozoom
	ContentLeft       string // LeftPos / ContentScale (for the zoom-based layout)
	ContentTop        string // TopHeader / ContentScale (for the zoom-based layout)
	LeftPos           int
	TopHeader         int // 69*autozoom
	Top72             int // 72*autozoom
	Bottom25          int // 25*autozoom
	Floors            []floorButton

	NoDescription    bool
	Desknumbers      bool
	Shownames        bool
	HighlightLeaders bool
	Printmode        bool
	NoAnimation      bool
	DailyVisitors    bool
	DisableSAML      bool
	UserMode         string
	UserModeColor    string
	AnnounceCookie   int

	LoggedIn  bool
	Username  string
	Fullname  string
	Phone     string
	Mail      string
	PermDesks int
	PermMaps  int
	PermAdmin int
	IsEditor  bool
	Token     string
	AvatarURL string

	DepartmentsJSON template.JS
	LogoRegular     string
	LogoHover       string
	TeamsContact    string
	Domain          string
	Region          string
	TzOffset        string

	Findme    string
	Teamlabel string

	SAMLEnabled        bool
	AllowLocalFallback bool
}

// renderIndex computes the page state and renders index.html.
func (app *App) renderIndex(w http.ResponseWriter, r *http.Request, sess Session, loggedIn bool) {
	q := r.URL.Query()

	mapDefault := "overview"
	mapName := q.Get("map")
	if mapName == "" {
		if c, err := r.Cookie("map"); err == nil {
			mapName = c.Value
		}
	}
	if mapName == "" {
		mapName = mapDefault
	}
	mapName = strings.ToLower(mapName)

	// Build the published map list (always include "overview").
	var mapList []string
	maps, _ := app.db.ListMaps()
	displayNames := make(map[string]string, len(maps))
	hasOverview := false
	for _, m := range maps {
		if m.DisplayName != "" {
			displayNames[m.Mapname] = m.DisplayName
		}
		if m.Published != "no" {
			mapList = append(mapList, m.Mapname)
		}
		if m.Mapname == "overview" {
			hasOverview = true
		}
	}
	if !hasOverview {
		mapList = append(mapList, "overview")
	}
	if !containsStr(mapList, mapName) {
		mapName = mapDefault
	}

	// Persist the selected map in a cookie (matches legacy behaviour).
	http.SetCookie(w, &http.Cookie{Name: "map", Value: mapName, Path: "/", SameSite: http.SameSiteLaxMode, Expires: time.Now().AddDate(5, 0, 0)})

	otherMaps := make([]mapLink, 0, len(mapList))
	for _, m := range mapList {
		if m != mapName {
			label := displayNames[m]
			if label == "" {
				label = ucfirst(m)
			}
			otherMaps = append(otherMaps, mapLink{Name: m, Label: label})
		}
	}

	// Item scale for this map.
	itemscale := "1"
	if mi, found, _ := app.db.GetMap(mapName); found && mi.Itemscale != "" && mi.Itemscale != "0" {
		itemscale = mi.Itemscale
	}

	// Zoom / autozoom / scroll position from cookies and query.
	autozoom := cookieInt(r, "autozoom", 1)
	zoom := cookieInt(r, "zoom", 100)
	if z := q.Get("zoom"); z != "" {
		if zi, err := strconv.Atoi(z); err == nil {
			zoom = zi
		}
	}
	if zoom < 10 {
		zoom = 10
	}
	if zoom > 100 {
		zoom = 100
	}
	http.SetCookie(w, &http.Cookie{Name: "zoom", Value: strconv.Itoa(zoom), Path: "/", Expires: time.Now().AddDate(5, 0, 0)})
	leftPos := cookieInt(r, "LeftPos", 0)

	targetWidth := 1600

	// Floor buttons for non-overview maps.
	var floors []floorButton
	if mapName != "overview" {
		desks, _ := app.db.ListDesks(mapName)
		for _, d := range desks {
			if strings.EqualFold(d.Desktype, "floor") {
				floors = append(floors, floorButton{ID: d.ID, Employee: d.Employee})
			}
		}
	}

	// Departments as a JSON object keyed by index (matches JSON_FORCE_OBJECT).
	deptObj := map[string]string{}
	depts, _ := app.db.ListDepartments()
	for i, d := range depts {
		deptObj[strconv.Itoa(i)] = d
	}
	deptJSON, _ := json.Marshal(deptObj)

	// Timezone region + offset.
	region := "Europe/Berlin"
	if mi, found, _ := app.db.GetMap(mapName); found && mi.Timezone != "" {
		region = mi.Timezone
	}
	tzOffset := "0"
	if loc, err := time.LoadLocation(region); err == nil {
		_, off := time.Now().In(loc).Zone()
		tzOffset = strconv.Itoa(off / 3600)
	}

	// Permissions.
	permDesks := app.permLevel(sess, "desks")
	permMaps := app.permLevel(sess, "maps")
	permAdmin := app.permLevel(sess, "adminpanel")
	isEditor := permDesks > 1

	token := ""
	if isEditor {
		ymd := time.Now().Format("20060102")
		rev := reverseStr(ymd)
		a, _ := strconv.Atoi(rev)
		b, _ := strconv.Atoi(ymd)
		token = strconv.Itoa(a + b)
	}

	// Avatar.
	avatarURL := "images/noavatar.png"
	if loggedIn {
		userid := sess.Username
		if i := strings.LastIndex(userid, "\\"); i >= 0 {
			userid = userid[i+1:]
		}
		if _, err := os.Stat(app.cfg.dataPath("avatarcache", userid+".jpg")); err == nil {
			avatarURL = "avatarcache/" + userid + ".jpg?time=" + strconv.FormatInt(time.Now().Unix(), 10)
		}
	}

	usermode := cookieStr(r, "setting_usermode", "edit")
	usermodeColor := "#333"
	if usermode == "user" {
		usermodeColor = "orange"
	}

	// Content scale + zoom-based positioning. The map is laid out at a fixed
	// design width and shown with CSS `zoom` (instead of transform:scale) so the
	// browser re-rasterizes text at the final size and fonts stay crisp. Because
	// `zoom` also scales an element's left/top offsets, we pre-divide them here.
	contentScale := float64(zoom) / 100 * float64(autozoom)
	if contentScale <= 0 {
		contentScale = 1
	}
	contentLeft := float64(leftPos) / contentScale
	contentTop := float64(69*autozoom) / contentScale

	findme := q.Get("findme")
	if xssCheck(findme) {
		findme = ""
	}

	mapTitle := displayNames[mapName]
	if mapTitle == "" {
		mapTitle = ucfirst(mapName)
	}

	data := indexData{
		AppTitle:          app.appTitle(),
		TargetScreenWidth: targetWidth,
		HalfWidth:         targetWidth / 2,
		Map:               mapName,
		MapTitle:          mapTitle,
		MapDefault:        mapDefault,
		MapList:           mapList,
		OtherMaps:         otherMaps,
		IsOverview:        mapName == "overview",
		Itemscale:         itemscale,
		Autozoom:          autozoom,
		Zoom:              zoom,
		ZoomIn:            zoom + 10,
		ZoomOut:           zoom - 10,
		ZoomScaled:        strconv.FormatFloat(float64(zoom)/100, 'f', -1, 64),
		ContentScale:      strconv.FormatFloat(float64(zoom)/100*float64(autozoom), 'f', -1, 64),
		ContentLeft:       strconv.FormatFloat(contentLeft, 'f', -1, 64),
		ContentTop:        strconv.FormatFloat(contentTop, 'f', -1, 64),
		LeftPos:           leftPos,
		TopHeader:         69 * autozoom,
		Top72:             72 * autozoom,
		Bottom25:          25 * autozoom,
		Floors:            floors,

		NoDescription:    cookieBool(r, "setting_nodescription"),
		Desknumbers:      cookieBool(r, "setting_desknumbers"),
		Shownames:        cookieBool(r, "setting_shownames"),
		HighlightLeaders: cookieBool(r, "setting_highlightleaders"),
		Printmode:        cookieBool(r, "setting_printmode"),
		NoAnimation:      cookieBool(r, "setting_noanimation"),
		DailyVisitors:    cookieBool(r, "setting_dailyvisitors"),
		DisableSAML:      cookieBool(r, "setting_saml"),
		UserMode:         usermode,
		UserModeColor:    usermodeColor,
		AnnounceCookie:   cookieInt(r, "announcecookie", 0),

		LoggedIn:  loggedIn,
		Username:  sess.Username,
		Fullname:  sess.Fullname,
		Phone:     sess.Phone,
		Mail:      sess.Mail,
		PermDesks: permDesks,
		PermMaps:  permMaps,
		PermAdmin: permAdmin,
		IsEditor:  isEditor,
		Token:     token,
		AvatarURL: avatarURL,

		DepartmentsJSON: template.JS(deptJSON),
		LogoRegular:     app.settingOr("logo_regular", "/static/images/cmaps-regular.png"),
		LogoHover:       app.settingOr("logo_hover", "/static/images/cmaps-hover.png"),
		TeamsContact:    app.db.GetSetting("teamsContact"),
		Domain:          app.db.GetSetting("domain"),
		Region:          region,
		TzOffset:        tzOffset,

		Findme:    findme,
		Teamlabel: q.Get("teamlabel"),

		SAMLEnabled:        app.cfg.SAML.Enabled,
		AllowLocalFallback: app.cfg.SAML.AllowLocalPasswordFallback,
	}

	app.render(w, "index.html", data)
}

// settingOr returns a DB setting or a default when empty.
func (app *App) settingOr(key, def string) string {
	if v := app.db.GetSetting(key); v != "" {
		return v
	}
	return def
}

func cookieInt(r *http.Request, name string, def int) int {
	c, err := r.Cookie(name)
	if err != nil {
		return def
	}
	v, err := strconv.Atoi(c.Value)
	if err != nil {
		return def
	}
	return v
}

func cookieBool(r *http.Request, name string) bool {
	c, err := r.Cookie(name)
	return err == nil && c.Value == "1"
}

func cookieStr(r *http.Request, name, def string) string {
	c, err := r.Cookie(name)
	if err != nil || c.Value == "" {
		return def
	}
	return c.Value
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func ucfirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func reverseStr(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// xssCheck mirrors the legacy search-box blacklist.
func xssCheck(s string) bool {
	for _, bad := range []string{"<", ">", "[", "]", "&amp;", "&lt;", "&gt;", "&quot;", "&#x27;", "&#x2F;"} {
		if strings.Contains(strings.ToLower(s), bad) {
			return true
		}
	}
	return false
}
