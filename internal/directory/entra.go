package directory

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
	"strings"
	"sync"
	"time"

	"companymaps/internal/progress"
	"companymaps/internal/store"
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

// EntraClient is a Microsoft Graph API client for a single Entra ID app
// registration, supporting both client-secret and certificate auth.
type EntraClient struct {
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

func newEntraSecretClient(tenantID, clientID, clientSecret string) *EntraClient {
	return &EntraClient{
		tenantID:     tenantID,
		clientID:     clientID,
		authMethod:   "secret",
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

func newEntraCertClient(tenantID, clientID, certPEM, keyPEM string) *EntraClient {
	return &EntraClient{
		tenantID:   tenantID,
		clientID:   clientID,
		authMethod: "certificate",
		certPEM:    certPEM,
		keyPEM:     keyPEM,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *EntraClient) AccessToken() (string, error) {
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

func (c *EntraClient) fetchTokenSecret() (string, time.Time, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID)
	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"scope":         {"https://graph.microsoft.com/.default"},
	}
	return c.postTokenRequest(tokenURL, body)
}

func (c *EntraClient) fetchTokenCert() (string, time.Time, error) {
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

func (c *EntraClient) postTokenRequest(tokenURL string, body url.Values) (string, time.Time, error) {
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

func (c *EntraClient) Get(path string, out any) error {
	token, err := c.AccessToken()
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

// EntraGraphUser is the subset of a Graph user object needed to build the same
// mirror shape as the LDAP sync.
type EntraGraphUser struct {
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
func (c *EntraClient) ListUsers() ([]EntraGraphUser, error) {
	var all []EntraGraphUser
	path := "/users?$select=" + entraUserSelect + "&$top=999"
	for path != "" {
		var page struct {
			Value    []EntraGraphUser `json:"value"`
			NextLink string           `json:"@odata.nextLink"`
		}
		if err := c.Get(path, &page); err != nil {
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

// graphUserToDirectory maps a Graph user object onto the store.DirectoryUser shape
// used by deriveMirrorUsers, so the EntraID sync produces exactly the same
// office-filtered mirror as the LDAP sync.
func (s *Syncer) GraphUserToDirectory(u EntraGraphUser) store.DirectoryUser {
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
	mail = store.NormalizeMail(mail)
	title := strings.TrimSpace(u.JobTitle)
	if title == "" {
		title = "-"
	}
	phone := ""
	if len(u.BusinessPhones) > 0 {
		phone = strings.TrimSpace(u.BusinessPhones[0])
	}
	return store.DirectoryUser{
		Userid:         UserIdentifier(s.DB, sam, mail),
		Samaccountname: sam,
		Givenname:      strings.TrimSpace(u.GivenName),
		Surname:        strings.TrimSpace(u.Surname),
		Mail:           mail,
		Office:         strings.TrimSpace(u.OfficeLocation),
		Department:     strings.TrimSpace(u.Department),
		Title:          title,
		Phone:          phone,
		Mobile:         strings.TrimSpace(u.MobilePhone),
		Aliases:        ExtractProxyAliases(u.ProxyAddresses, mail),
	}
}

// entraHasEnabledSource reports whether at least one enabled EntraID source is
// configured, so the scheduler and sync endpoint can skip cleanly otherwise.
func (s *Syncer) EntraHasEnabledSource() bool {
	srcs, _ := s.DB.ListEntraSources()
	for _, s := range srcs {
		if !s.Disabled {
			return true
		}
	}
	return false
}

// entraSourceConfigured reports whether a source has the minimum credentials to
// attempt a sync.
func EntraSourceConfigured(s store.EntraSource) bool {
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
func NewEntraClient(s store.EntraSource) (*EntraClient, error) {
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
// (stored as individual settings) into one store.EntraSource record the first time the
// new multi-connection model runs, so existing setups keep working. Legacy
// settings are left in place; they are simply no longer the source of truth.
func (s *Syncer) MigrateEntraConfig() {
	if srcs, _ := s.DB.ListEntraSources(); len(srcs) > 0 {
		return
	}
	tenant := strings.TrimSpace(s.DB.GetEntraSetting("entraTenantID"))
	client := strings.TrimSpace(s.DB.GetEntraSetting("entraClientID"))
	if tenant == "" || client == "" {
		return // nothing configured to migrate
	}
	method := s.DB.GetEntraSetting("entraAuthMethod")
	if method != "certificate" {
		method = "secret"
	}
	lastSync := s.DB.GetEntraSetting("entraLastSync")
	if lastSync == "" {
		lastSync = "never"
	}
	src := store.EntraSource{
		ID:           1,
		Description:  "EntraID",
		TenantID:     tenant,
		ClientID:     client,
		AuthMethod:   method,
		ClientSecret: s.DB.GetEntraSetting("entraClientSecret"),
		CertPEM:      s.DB.GetEntraSetting("entraCertPem"),
		KeyPEM:       s.DB.GetEntraSetting("entraKeyPem"),
		LastSync:     lastSync,
		Disabled:     s.DB.GetEntraSetting("entraEnabled") == "0",
	}
	if err := s.DB.PutEntraSource(src); err != nil {
		log.Printf("EntraID config migration failed: %v", err)
	}
}

// RunEntraSync runs a full EntraID sync without progress reporting.
func (s *Syncer) RunEntraSync() (int, error) {
	return s.RunEntraSyncProg(nil)
}

// runEntraSync fetches all users from every enabled EntraID source, derives the
// office-filtered desk-placement mirror for each and stores it in that source's
// own bucket, then rebuilds the combined EntraID mirror from all enabled
// sources (combine-on-write), so a single-source sync never wipes the others.
func (s *Syncer) RunEntraSyncProg(prog *progress.Progress) (int, error) {
	sources, err := s.DB.ListEntraSources()
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
		prog.SetTotal(len(sources))
		prog.Logf("Starting sync of %d EntraID source(s)…", len(sources))
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, src := range sources {
		if prog != nil {
			prog.SetStage("Syncing " + src.Description)
			prog.Logf("→ %s: connecting to Microsoft Graph…", src.Description)
		}
		client, err := NewEntraClient(src)
		if err != nil {
			if prog != nil {
				prog.Logf("   ✗ %s: %s", src.Description, err.Error())
				prog.Step("")
			}
			return 0, fmt.Errorf("source %q: %w", src.Description, err)
		}
		users, err := client.ListUsers()
		if err != nil {
			if prog != nil {
				prog.Logf("   ✗ %s: %s", src.Description, err.Error())
				prog.Step("")
			}
			return 0, fmt.Errorf("source %q: graph list users: %w", src.Description, err)
		}

		dir := make([]store.DirectoryUser, 0, len(users))
		for _, u := range users {
			if !u.AccountEnabled {
				continue
			}
			dir = append(dir, s.GraphUserToDirectory(u))
		}
		mirror := DeriveMirrorUsers(dir)
		if err := s.DB.PutSourceMirror("entra", src.ID, mirror); err != nil {
			log.Printf("EntraID sync: writing source mirror for %q: %v", src.Description, err)
		}

		src.LastSync = now
		if err := s.DB.PutEntraSource(src); err != nil {
			log.Printf("EntraID sync: updating LastSync for %q: %v", src.Description, err)
		}
		if prog != nil {
			prog.Logf("   %d user(s), %d desk placement(s).", len(dir), len(mirror))
			prog.Step("")
		}
	}

	count, err := s.RebuildEntraMirror()
	if err != nil {
		return count, err
	}
	_ = s.DB.SetMeta("entraSeeded", "1")
	_ = s.DB.SetEntraSetting("entraLastSync", now)
	if prog != nil {
		prog.Logf("Done. %d desk placement(s) from %d source(s).", count, len(sources))
	}
	return count, nil
}

// rebuildEntraMirror recombines every enabled EntraID source's per-source mirror
// into the shared EntraID mirror cache (combine-on-write). EntraID has no
// change detection, so no changelog announcements are produced.
func (s *Syncer) RebuildEntraMirror() (int, error) {
	sources, err := s.DB.ListEntraSources()
	if err != nil {
		return 0, fmt.Errorf("loading EntraID sources: %w", err)
	}
	var combined []store.LdapUser
	for _, src := range sources {
		if src.Disabled {
			continue
		}
		users, _ := s.DB.GetSourceMirror("entra", src.ID)
		combined = append(combined, users...)
	}
	avatars := s.AvatarFileSet()
	for i := range combined {
		combined[i].HasAvatar = avatars[strings.ToLower(combined[i].Userid)]
	}
	if err := s.DB.ReplaceEntraLdap(combined); err != nil {
		return len(combined), fmt.Errorf("writing EntraID mirror: %w", err)
	}
	return len(combined), nil
}

// generateEntraCert creates a 2048-bit RSA key pair and a self-signed X.509
// certificate valid for ~3 years, returning both as PEM strings.
func GenerateEntraCert() (string, string, error) {
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
