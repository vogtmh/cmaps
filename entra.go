package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
func graphUserToDirectory(u entraGraphUser) DirectoryUser {
	userid := strings.TrimSpace(u.OnPremisesSamAccountName)
	if userid == "" {
		// Fall back to the UPN local part (before "@") when the account is not
		// synced from on-prem AD.
		upn := strings.TrimSpace(u.UserPrincipalName)
		if at := strings.Index(upn, "@"); at > 0 {
			userid = upn[:at]
		} else {
			userid = upn
		}
	}
	mail := strings.TrimSpace(u.Mail)
	if mail == "" {
		mail = strings.TrimSpace(u.UserPrincipalName)
	}
	title := strings.TrimSpace(u.JobTitle)
	if title == "" {
		title = "-"
	}
	phone := ""
	if len(u.BusinessPhones) > 0 {
		phone = strings.TrimSpace(u.BusinessPhones[0])
	}
	return DirectoryUser{
		Userid:     userid,
		Givenname:  strings.TrimSpace(u.GivenName),
		Surname:    strings.TrimSpace(u.Surname),
		Mail:       mail,
		Office:     strings.TrimSpace(u.OfficeLocation),
		Department: strings.TrimSpace(u.Department),
		Title:      title,
		Phone:      phone,
		Mobile:     strings.TrimSpace(u.MobilePhone),
		Aliases:    extractProxyAliases(u.ProxyAddresses, mail),
	}
}

// entraConfigured reports whether the EntraID connection has the minimum
// settings needed to attempt a sync.
func (app *App) entraConfigured() bool {
	tenant := strings.TrimSpace(app.db.GetEntraSetting("entraTenantID"))
	client := strings.TrimSpace(app.db.GetEntraSetting("entraClientID"))
	if tenant == "" || client == "" {
		return false
	}
	switch app.db.GetEntraSetting("entraAuthMethod") {
	case "certificate":
		return strings.TrimSpace(app.db.GetEntraSetting("entraCertPem")) != "" &&
			strings.TrimSpace(app.db.GetEntraSetting("entraKeyPem")) != ""
	default: // secret
		return strings.TrimSpace(app.db.GetEntraSetting("entraClientSecret")) != ""
	}
}

// newEntraClientFromSettings builds a Graph client from the stored settings.
func (app *App) newEntraClientFromSettings() (*entraClient, error) {
	tenant := strings.TrimSpace(app.db.GetEntraSetting("entraTenantID"))
	client := strings.TrimSpace(app.db.GetEntraSetting("entraClientID"))
	if tenant == "" || client == "" {
		return nil, fmt.Errorf("EntraID tenant and client id are required")
	}
	method := app.db.GetEntraSetting("entraAuthMethod")
	switch method {
	case "certificate":
		cert := strings.TrimSpace(app.db.GetEntraSetting("entraCertPem"))
		key := strings.TrimSpace(app.db.GetEntraSetting("entraKeyPem"))
		if cert == "" || key == "" {
			return nil, fmt.Errorf("EntraID certificate and private key are required for certificate auth")
		}
		return newEntraCertClient(tenant, client, cert, key), nil
	default: // secret
		secret := strings.TrimSpace(app.db.GetEntraSetting("entraClientSecret"))
		if secret == "" {
			return nil, fmt.Errorf("EntraID client secret is required for secret auth")
		}
		return newEntraSecretClient(tenant, client, secret), nil
	}
}

// RunEntraSync runs a full EntraID sync without progress reporting.
func (app *App) RunEntraSync() (int, error) {
	return app.runEntraSync(nil)
}

// runEntraSync fetches all users from Microsoft Graph, derives the
// office-filtered desk-placement mirror, and stores it in the EntraID bucket.
func (app *App) runEntraSync(prog *syncProgress) (int, error) {
	if prog != nil {
		prog.logf("Connecting to Microsoft Graph…")
	}
	client, err := app.newEntraClientFromSettings()
	if err != nil {
		return 0, err
	}
	if prog != nil {
		prog.logf("Fetching users…")
	}
	users, err := client.listUsers()
	if err != nil {
		return 0, fmt.Errorf("graph list users: %w", err)
	}
	if prog != nil {
		prog.logf("Fetched %d user(s). Deriving desk placements…", len(users))
	}

	dir := make([]DirectoryUser, 0, len(users))
	for _, u := range users {
		if !u.AccountEnabled {
			continue
		}
		dir = append(dir, graphUserToDirectory(u))
	}

	mirror := deriveMirrorUsers(dir)

	// Flag which mirrored users have a cached avatar on disk, mirroring the LDAP
	// sync so the client can point users without one at a shared placeholder.
	avatars := app.avatarFileSet()
	for i := range mirror {
		mirror[i].HasAvatar = avatars[strings.ToLower(mirror[i].Userid)]
	}

	if err := app.db.ReplaceEntraLdap(mirror); err != nil {
		return len(mirror), fmt.Errorf("writing EntraID mirror: %w", err)
	}
	_ = app.db.SetEntraSetting("entraLastSync", time.Now().Format("2006-01-02 15:04:05"))
	if prog != nil {
		prog.logf("Done. %d directory user(s), %d desk placement(s).", len(dir), len(mirror))
	}
	return len(mirror), nil
}

// StartEntraSyncScheduler runs RunEntraSync in the background on a fixed
// interval. Sync is skipped when the EntraID connection is not configured.
func (app *App) StartEntraSyncScheduler(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if !app.entraConfigured() {
				continue
			}
			if n, err := app.RunEntraSync(); err != nil {
				log.Printf("scheduled EntraID sync failed: %v", err)
			} else {
				log.Printf("scheduled EntraID sync: %d placements mirrored", n)
			}
		}
	}()
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

// handleRestEntraTest validates the stored EntraID credentials by acquiring a
// token and making a minimal Microsoft Graph call, without running a full sync.
func (app *App) handleRestEntraTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	client, err := app.newEntraClientFromSettings()
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	var page struct {
		Count int              `json:"@odata.count"`
		Value []entraGraphUser `json:"value"`
	}
	if err := client.get("/users?$top=1&$count=true&$select=id,displayName", &page); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	msg := "Connection successful \u2014 token acquired and Microsoft Graph reachable."
	if page.Count > 0 {
		msg = fmt.Sprintf("Connection successful \u2014 %d user(s) visible in the tenant.", page.Count)
	}
	writeJSON(w, map[string]interface{}{"ok": true, "message": msg})
}
