package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EntraID (Microsoft Graph) directory sync. This is a second directory source
// alongside LDAP/AD. It fetches users from a single Entra ID app registration
// via the OAuth 2.0 client-credentials flow (client secret or certificate),
// derives the same office-filtered desk-placement mirror as the LDAP sync
// (reusing deriveMirrorUsers), and stores it in a SEPARATE bucket so the two can
// be compared side by side without either overwriting the other.
//
// The Graph client is a minimal port of misc/entraapi/graph and has no
// third-party dependencies beyond the Go standard library.

// ──────────────────────────────────────────────
// Graph client
// ──────────────────────────────────────────────

type entraCachedToken struct {
	accessToken string
	expiresAt   time.Time
}

// entraClient is a Microsoft Graph API client for a single Entra ID app
// registration, supporting both client-secret and certificate auth.
type entraClient struct {
	tenantID     string
	clientID     string
	authMethod   string // "secret" or "certificate"
	clientSecret string // used when authMethod == "secret"
	certPEM      string // PEM certificate, used when authMethod == "certificate"
	keyPEM       string // PEM private key

	mu    sync.Mutex
	token *entraCachedToken
	http  *http.Client
}

func newEntraSecretClient(tenantID, clientID, clientSecret string) *entraClient {
	return &entraClient{
		tenantID:     tenantID,
		clientID:     clientID,
		authMethod:   "secret",
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

func newEntraCertClient(tenantID, clientID, certPEM, keyPEM string) *entraClient {
	return &entraClient{
		tenantID:   tenantID,
		clientID:   clientID,
		authMethod: "certificate",
		certPEM:    certPEM,
		keyPEM:     keyPEM,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *entraClient) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != nil && time.Now().Before(c.token.expiresAt) {
		return c.token.accessToken, nil
	}

	var token string
	var expires time.Time
	var err error

	switch c.authMethod {
	case "secret":
		token, expires, err = c.fetchTokenSecret()
	case "certificate":
		token, expires, err = c.fetchTokenCert()
	default:
		return "", fmt.Errorf("unknown auth method: %s", c.authMethod)
	}
	if err != nil {
		return "", err
	}

	c.token = &entraCachedToken{accessToken: token, expiresAt: expires}
	return token, nil
}

func (c *entraClient) fetchTokenSecret() (string, time.Time, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID)
	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"scope":         {"https://graph.microsoft.com/.default"},
	}
	return c.postTokenRequest(tokenURL, body)
}

func (c *entraClient) fetchTokenCert() (string, time.Time, error) {
	// Parse certificate to get thumbprint for x5t header.
	certBlock, _ := pem.Decode([]byte(c.certPEM))
	if certBlock == nil {
		return "", time.Time{}, fmt.Errorf("invalid certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse certificate: %w", err)
	}
	thumbprint := sha1.Sum(cert.Raw)
	x5t := base64.StdEncoding.EncodeToString(thumbprint[:])

	// Parse private key.
	keyBlock, _ := pem.Decode([]byte(c.keyPEM))
	if keyBlock == nil {
		return "", time.Time{}, fmt.Errorf("invalid key PEM")
	}
	var privateKey *rsa.PrivateKey
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		key, e := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if e != nil {
			return "", time.Time{}, fmt.Errorf("parse PKCS8 key: %w", e)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return "", time.Time{}, fmt.Errorf("only RSA private keys are supported")
		}
	default:
		return "", time.Time{}, fmt.Errorf("unsupported key type: %s", keyBlock.Type)
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse private key: %w", err)
	}

	// Build JWT client assertion (RS256).
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID)

	now := time.Now()
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"x5t": x5t,
	}
	claims := map[string]any{
		"aud": tokenURL,
		"iss": c.clientID,
		"sub": c.clientID,
		"jti": entraRandomJTI(),
		"nbf": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	hash := crypto.SHA256.New()
	hash.Write([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash.Sum(nil))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}
	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	body := url.Values{
		"grant_type":            {"client_credentials"},
		"client_id":             {c.clientID},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {assertion},
		"scope":                 {"https://graph.microsoft.com/.default"},
	}
	return c.postTokenRequest(tokenURL, body)
}

func (c *entraClient) postTokenRequest(tokenURL string, body url.Values) (string, time.Time, error) {
	resp, err := c.http.PostForm(tokenURL, body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("parse token response: %w", err)
	}
	if result.Error != "" {
		return "", time.Time{}, fmt.Errorf("token error %s: %s", result.Error, result.ErrorDesc)
	}
	// Expire 60s early to account for clock skew.
	expires := time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return result.AccessToken, expires, nil
}

func entraRandomJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (c *entraClient) get(path string, out any) error {
	token, err := c.accessToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, "https://graph.microsoft.com/v1.0"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("ConsistencyLevel", "eventual")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("graph GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("graph GET %s: HTTP %d: %s", path, resp.StatusCode, string(raw))
	}
	return json.Unmarshal(raw, out)
}

// entraGraphUser is the subset of a Graph user object needed to build the same
// mirror shape as the LDAP sync.
type entraGraphUser struct {
	ID                       string   `json:"id"`
	DisplayName              string   `json:"displayName"`
	GivenName                string   `json:"givenName"`
	Surname                  string   `json:"surname"`
	UserPrincipalName        string   `json:"userPrincipalName"`
	Mail                     string   `json:"mail"`
	JobTitle                 string   `json:"jobTitle"`
	Department               string   `json:"department"`
	OfficeLocation           string   `json:"officeLocation"`
	MobilePhone              string   `json:"mobilePhone"`
	BusinessPhones           []string `json:"businessPhones"`
	ProxyAddresses           []string `json:"proxyAddresses"`
	AccountEnabled           bool     `json:"accountEnabled"`
	OnPremisesSamAccountName string   `json:"onPremisesSamAccountName"`
}

// entraUserSelect is the $select field list used when fetching users, matching
// the attributes the mirror derivation and comparison consume.
const entraUserSelect = "id,displayName,givenName,surname,userPrincipalName,mail,jobTitle,department,officeLocation,mobilePhone,businessPhones,proxyAddresses,accountEnabled,onPremisesSamAccountName"

// listUsers pages through all users in the tenant.
func (c *entraClient) listUsers() ([]entraGraphUser, error) {
	var all []entraGraphUser
	path := "/users?$select=" + entraUserSelect + "&$top=999"
	for path != "" {
		var page struct {
			Value    []entraGraphUser `json:"value"`
			NextLink string           `json:"@odata.nextLink"`
		}
		if err := c.get(path, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Value...)
		if page.NextLink != "" {
			path = strings.TrimPrefix(page.NextLink, "https://graph.microsoft.com/v1.0")
		} else {
			path = ""
		}
	}
	return all, nil
}

// ──────────────────────────────────────────────
// Conversion + sync
// ──────────────────────────────────────────────

// graphUserToDirectory maps a Graph user object onto the DirectoryUser shape
// used by deriveMirrorUsers, so the EntraID sync produces exactly the same
// office-filtered mirror as the LDAP sync.
func (app *App) graphUserToDirectory(u entraGraphUser) DirectoryUser {
	sam := strings.TrimSpace(u.OnPremisesSamAccountName)
	if sam == "" {
		// Fall back to the UPN local part (before "@") when the account is not
		// synced from on-prem AD.
		upn := strings.TrimSpace(u.UserPrincipalName)
		if at := strings.Index(upn, "@"); at > 0 {
			sam = upn[:at]
		} else {
			sam = upn
		}
	}
	mail := strings.TrimSpace(u.Mail)
	if mail == "" {
		mail = strings.TrimSpace(u.UserPrincipalName)
	}
	mail = normalizeMail(mail)
	title := strings.TrimSpace(u.JobTitle)
	if title == "" {
		title = "-"
	}
	phone := ""
	if len(u.BusinessPhones) > 0 {
		phone = strings.TrimSpace(u.BusinessPhones[0])
	}
	return DirectoryUser{
		Userid:         app.userIdentifier(sam, mail),
		Samaccountname: sam,
		Givenname:      strings.TrimSpace(u.GivenName),
		Surname:        strings.TrimSpace(u.Surname),
		Mail:       mail,
		Office:     strings.TrimSpace(u.OfficeLocation),
		Department: strings.TrimSpace(u.Department),
		Title:      title,
		Phone:      phone,
		Mobile:     strings.TrimSpace(u.MobilePhone),
		Aliases:    extractProxyAliases(u.ProxyAddresses, mail),
	}
}

// entraHasEnabledSource reports whether at least one enabled EntraID source is
// configured, so the scheduler and sync endpoint can skip cleanly otherwise.
func (app *App) entraHasEnabledSource() bool {
	srcs, _ := app.db.ListEntraSources()
	for _, s := range srcs {
		if !s.Disabled {
			return true
		}
	}
	return false
}

// entraSourceConfigured reports whether a source has the minimum credentials to
// attempt a sync.
func entraSourceConfigured(s EntraSource) bool {
	if strings.TrimSpace(s.TenantID) == "" || strings.TrimSpace(s.ClientID) == "" {
		return false
	}
	switch s.AuthMethod {
	case "certificate":
		return strings.TrimSpace(s.CertPEM) != "" && strings.TrimSpace(s.KeyPEM) != ""
	default: // secret
		return strings.TrimSpace(s.ClientSecret) != ""
	}
}

// newEntraClient builds a Graph client for a single EntraID source.
func newEntraClient(s EntraSource) (*entraClient, error) {
	tenant := strings.TrimSpace(s.TenantID)
	client := strings.TrimSpace(s.ClientID)
	if tenant == "" || client == "" {
		return nil, fmt.Errorf("EntraID tenant and client id are required")
	}
	switch s.AuthMethod {
	case "certificate":
		cert := strings.TrimSpace(s.CertPEM)
		key := strings.TrimSpace(s.KeyPEM)
		if cert == "" || key == "" {
			return nil, fmt.Errorf("EntraID certificate and private key are required for certificate auth")
		}
		return newEntraCertClient(tenant, client, cert, key), nil
	default: // secret
		secret := strings.TrimSpace(s.ClientSecret)
		if secret == "" {
			return nil, fmt.Errorf("EntraID client secret is required for secret auth")
		}
		return newEntraSecretClient(tenant, client, secret), nil
	}
}

// migrateEntraConfig converts the legacy single-connection EntraID configuration
// (stored as individual settings) into one EntraSource record the first time the
// new multi-connection model runs, so existing setups keep working. Legacy
// settings are left in place; they are simply no longer the source of truth.
func (app *App) migrateEntraConfig() {
	if srcs, _ := app.db.ListEntraSources(); len(srcs) > 0 {
		return
	}
	tenant := strings.TrimSpace(app.db.GetEntraSetting("entraTenantID"))
	client := strings.TrimSpace(app.db.GetEntraSetting("entraClientID"))
	if tenant == "" || client == "" {
		return // nothing configured to migrate
	}
	method := app.db.GetEntraSetting("entraAuthMethod")
	if method != "certificate" {
		method = "secret"
	}
	lastSync := app.db.GetEntraSetting("entraLastSync")
	if lastSync == "" {
		lastSync = "never"
	}
	src := EntraSource{
		ID:           1,
		Description:  "EntraID",
		TenantID:     tenant,
		ClientID:     client,
		AuthMethod:   method,
		ClientSecret: app.db.GetEntraSetting("entraClientSecret"),
		CertPEM:      app.db.GetEntraSetting("entraCertPem"),
		KeyPEM:       app.db.GetEntraSetting("entraKeyPem"),
		LastSync:     lastSync,
		Disabled:     app.db.GetEntraSetting("entraEnabled") == "0",
	}
	if err := app.db.PutEntraSource(src); err != nil {
		log.Printf("EntraID config migration failed: %v", err)
	}
}

// RunEntraSync runs a full EntraID sync without progress reporting.
func (app *App) RunEntraSync() (int, error) {
	return app.runEntraSync(nil)
}

// runEntraSync fetches all users from every enabled EntraID source, derives the
// office-filtered desk-placement mirror for each and stores it in that source's
// own bucket, then rebuilds the combined EntraID mirror from all enabled
// sources (combine-on-write), so a single-source sync never wipes the others.
func (app *App) runEntraSync(prog *syncProgress) (int, error) {
	sources, err := app.db.ListEntraSources()
	if err != nil {
		return 0, fmt.Errorf("loading EntraID sources: %w", err)
	}
	enabled := sources[:0]
	for _, s := range sources {
		if !s.Disabled {
			enabled = append(enabled, s)
		}
	}
	sources = enabled
	if len(sources) == 0 {
		return 0, fmt.Errorf("no enabled EntraID sources configured")
	}
	if prog != nil {
		prog.setTotal(len(sources))
		prog.logf("Starting sync of %d EntraID source(s)…", len(sources))
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, src := range sources {
		if prog != nil {
			prog.setStage("Syncing " + src.Description)
			prog.logf("→ %s: connecting to Microsoft Graph…", src.Description)
		}
		client, err := newEntraClient(src)
		if err != nil {
			if prog != nil {
				prog.logf("   ✗ %s: %s", src.Description, err.Error())
				prog.step("")
			}
			return 0, fmt.Errorf("source %q: %w", src.Description, err)
		}
		users, err := client.listUsers()
		if err != nil {
			if prog != nil {
				prog.logf("   ✗ %s: %s", src.Description, err.Error())
				prog.step("")
			}
			return 0, fmt.Errorf("source %q: graph list users: %w", src.Description, err)
		}

		dir := make([]DirectoryUser, 0, len(users))
		for _, u := range users {
			if !u.AccountEnabled {
				continue
			}
			dir = append(dir, app.graphUserToDirectory(u))
		}
		mirror := deriveMirrorUsers(dir)
		if err := app.db.PutSourceMirror("entra", src.ID, mirror); err != nil {
			log.Printf("EntraID sync: writing source mirror for %q: %v", src.Description, err)
		}

		src.LastSync = now
		if err := app.db.PutEntraSource(src); err != nil {
			log.Printf("EntraID sync: updating LastSync for %q: %v", src.Description, err)
		}
		if prog != nil {
			prog.logf("   %d user(s), %d desk placement(s).", len(dir), len(mirror))
			prog.step("")
		}
	}

	count, err := app.rebuildEntraMirror()
	if err != nil {
		return count, err
	}
	_ = app.db.SetMeta("entraSeeded", "1")
	_ = app.db.SetEntraSetting("entraLastSync", now)
	if prog != nil {
		prog.logf("Done. %d desk placement(s) from %d source(s).", count, len(sources))
	}
	return count, nil
}

// rebuildEntraMirror recombines every enabled EntraID source's per-source mirror
// into the shared EntraID mirror cache (combine-on-write). EntraID has no
// change detection, so no changelog announcements are produced.
func (app *App) rebuildEntraMirror() (int, error) {
	sources, err := app.db.ListEntraSources()
	if err != nil {
		return 0, fmt.Errorf("loading EntraID sources: %w", err)
	}
	var combined []LdapUser
	for _, src := range sources {
		if src.Disabled {
			continue
		}
		users, _ := app.db.GetSourceMirror("entra", src.ID)
		combined = append(combined, users...)
	}
	avatars := app.avatarFileSet()
	for i := range combined {
		combined[i].HasAvatar = avatars[strings.ToLower(combined[i].Userid)]
	}
	if err := app.db.ReplaceEntraLdap(combined); err != nil {
		return len(combined), fmt.Errorf("writing EntraID mirror: %w", err)
	}
	return len(combined), nil
}

// ──────────────────────────────────────────────
// REST handlers
// ──────────────────────────────────────────────

// handleRestEntraSync starts an EntraID sync in the background so the admin Sync
// tab can poll for live progress.
func (app *App) handleRestEntraSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.entraHasEnabledSource() {
		writeJSON(w, map[string]interface{}{"started": false, "error": "No enabled EntraID connection."})
		return
	}
	if !app.entraProg.start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Manual EntraID sync")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.entraProg.finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		count, err := app.runEntraSync(&app.entraProg)
		if err != nil {
			app.entraProg.finish("", err.Error())
			return
		}
		app.entraProg.finish(fmt.Sprintf("Mirrored %d placement(s).", count), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestEntraProgress returns the current EntraID sync progress snapshot.
func (app *App) handleRestEntraProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.entraProg.snapshot())
}

// handleRestEntraTest validates one EntraID source's credentials by acquiring a
// token and making a minimal Microsoft Graph call, without running a full sync.
func (app *App) handleRestEntraTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("entraid"))
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "checks": []testCheck{{Name: "Connection", Status: "fail", Detail: "invalid EntraID id"}}})
		return
	}
	writeJSON(w, app.entraValidate(id))
}

// entraValidate runs a structured, read-only connectivity test for a single
// EntraID connection and returns the {ok, checks} payload rendered by the admin
// test modal. It acquires an access token and confirms Microsoft Graph is
// reachable — without performing a full sync.
func (app *App) entraValidate(id int) map[string]interface{} {
	var checks []testCheck
	add := func(name, status, detail string) {
		checks = append(checks, testCheck{Name: name, Status: status, Detail: detail})
	}

	srcs, _ := app.db.ListEntraSources()
	var src *EntraSource
	for i := range srcs {
		if srcs[i].ID == id {
			src = &srcs[i]
			break
		}
	}
	if src == nil {
		return testResult([]testCheck{{Name: "Connection", Status: "fail", Detail: "EntraID connection not found."}})
	}

	if src.Disabled {
		add("Connection status", "warn", "This connection is disabled and is skipped during sync.")
	} else {
		add("Connection status", "ok", "This connection is enabled.")
	}

	client, err := newEntraClient(*src)
	if err != nil {
		add("Access token", "fail", "Could not build the Graph client: "+err.Error())
		return testResult(checks)
	}
	if _, err := client.accessToken(); err != nil {
		add("Access token", "fail", "Could not acquire an access token: "+err.Error())
		return testResult(checks)
	}
	add("Access token", "ok", "Access token acquired for tenant "+src.TenantID+".")

	var page struct {
		Count int              `json:"@odata.count"`
		Value []entraGraphUser `json:"value"`
	}
	if err := client.get("/users?$top=1&$count=true&$select=id,displayName", &page); err != nil {
		add("Microsoft Graph", "fail", "Graph request failed: "+err.Error())
		return testResult(checks)
	}
	add("Microsoft Graph", "ok", "Microsoft Graph is reachable.")

	if page.Count > 0 {
		add("Users visible", "ok", fmt.Sprintf("%d user(s) visible in the tenant.", page.Count))
	} else {
		add("Users visible", "warn", "No users were returned — check the app registration's permissions and scope.")
	}

	return testResult(checks)
}

// handleRestEntraSyncOne synchronously syncs a single EntraID connection (the
// per-connection "Sync now" button), mirroring the LDAP per-source sync: the
// EntraID mirror is replaced with just this source's users.
func (app *App) handleRestEntraSyncOne(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("entraid"))
	if err != nil {
		http.Error(w, "invalid EntraID id", http.StatusBadRequest)
		return
	}
	srcs, _ := app.db.ListEntraSources()
	var src *EntraSource
	for i := range srcs {
		if srcs[i].ID == id {
			src = &srcs[i]
			break
		}
	}
	if src == nil {
		http.Error(w, "EntraID source not found", http.StatusNotFound)
		return
	}
	if src.Disabled {
		http.Error(w, "connection is disabled", http.StatusConflict)
		return
	}
	// If the per-source buckets have not been seeded yet (fresh upgrade), fall
	// back to a full sync so we never publish a mirror built from just one
	// source (which would drop the others until their next sync).
	if app.db.GetMeta("entraSeeded") != "1" {
		count, err := app.RunEntraSync()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Manual EntraID sync of source "+strconv.Itoa(id))
		writeJSON(w, map[string]interface{}{"status": "ok", "count": count, "lastSync": nowTimestamp()})
		return
	}
	client, err := newEntraClient(*src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	users, err := client.listUsers()
	if err != nil {
		http.Error(w, "graph list users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	dir := make([]DirectoryUser, 0, len(users))
	for _, u := range users {
		if !u.AccountEnabled {
			continue
		}
		dir = append(dir, app.graphUserToDirectory(u))
	}
	mirror := deriveMirrorUsers(dir)
	if err := app.db.PutSourceMirror("entra", src.ID, mirror); err != nil {
		http.Error(w, "writing EntraID mirror: "+err.Error(), http.StatusInternalServerError)
		return
	}
	count, err := app.rebuildEntraMirror()
	if err != nil {
		http.Error(w, "writing EntraID mirror: "+err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	src.LastSync = now
	_ = app.db.PutEntraSource(*src)
	_ = app.db.SetEntraSetting("entraLastSync", now)
	_ = app.db.AuditLog("LDAP", sess.Username, "Manual EntraID sync of source "+strconv.Itoa(id))
	writeJSON(w, map[string]interface{}{"status": "ok", "count": count, "lastSync": now})
}

// handleRestEntraGenCert generates a fresh self-signed RSA certificate and
// private key (both PEM-encoded) for use with EntraID certificate auth. The
// admin uploads the certificate to the app registration and keeps the private
// key here. Nothing is persisted server-side — the caller fills the form.
func (app *App) handleRestEntraGenCert(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	certPEM, keyPEM, err := generateEntraCert()
	if err != nil {
		writeJSON(w, map[string]interface{}{"status": "error", "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "cert": certPEM, "key": keyPEM})
}

// generateEntraCert creates a 2048-bit RSA key pair and a self-signed X.509
// certificate valid for ~3 years, returning both as PEM strings.
func generateEntraCert() (string, string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("generate serial: %w", err)
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "CompanyMaps EntraID Sync"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(3, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM), nil
}
