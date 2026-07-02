package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configFile = "config.json"

// Config holds all externally-configurable settings, persisted to config.json
// next to the executable. Everything else (maps, desks, users, ldap mirror,
// bookings, settings, secrets like the LDAP bind password and Robin token) lives
// in the boltDB inside the data directory and is editable from the admin UI.
type Config struct {
	ListenAddr    string     `json:"listen_addr"`
	AdminPassword string     `json:"admin_password"`
	DataDir       string     `json:"data_dir"`
	SAML          SAMLConfig `json:"saml"`
}

// SAMLConfig holds the SAML SP/IdP configuration. The SP is mounted at the exact
// legacy SimpleSAMLphp paths (see saml.go) so an existing Entra app registration
// requires no changes.
type SAMLConfig struct {
	Enabled                    bool   `json:"enabled"`
	AllowLocalPasswordFallback bool   `json:"allow_local_password_fallback"`
	FederationMetadataURL      string `json:"federation_metadata_url,omitempty"`
	EntraTenantID              string `json:"entra_tenant_id,omitempty"`
	EntraEntityID              string `json:"entra_entity_id,omitempty"`
	EntraLoginURL              string `json:"entra_login_url,omitempty"`
	EntraX509Certificate       string `json:"entra_x509_certificate,omitempty"`
	AppEntityID                string `json:"app_entity_id,omitempty"`
	AppReplyURL                string `json:"app_reply_url,omitempty"`
	AppLogoutURL               string `json:"app_logout_url,omitempty"`
	NameIDFormat               string `json:"name_id_format,omitempty"`

	// Attribute name overrides. When empty the Azure AD defaults used by the
	// original CompanyMaps PHP app are applied (see attribute*Default constants).
	AttributeSamAccount string `json:"attribute_samaccountname,omitempty"`
	AttributeGivenName  string `json:"attribute_givenname,omitempty"`
	AttributeSurname    string `json:"attribute_surname,omitempty"`
	AttributeFullName   string `json:"attribute_fullname,omitempty"`
	AttributeMail       string `json:"attribute_mail,omitempty"`
}

// Azure AD claim defaults, matching rest/account/index.php of the PHP app.
const (
	attrSamAccountDefault = "SamAccountName"
	attrGivenNameDefault  = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
	attrSurnameDefault    = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
	attrFullNameDefault   = "http://schemas.microsoft.com/identity/claims/displayname"
	// Use the dedicated e-mail claim (not the "name" claim, which is the UPN and
	// may differ from the mailbox on some IdPs). In mail identifier mode this is
	// the login identity, so it must be an unambiguous e-mail address.
	attrMailDefault = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
)

func (s SAMLConfig) attrSamAccount() string {
	return orDefault(s.AttributeSamAccount, attrSamAccountDefault)
}
func (s SAMLConfig) attrGivenName() string {
	return orDefault(s.AttributeGivenName, attrGivenNameDefault)
}
func (s SAMLConfig) attrSurname() string  { return orDefault(s.AttributeSurname, attrSurnameDefault) }
func (s SAMLConfig) attrFullName() string { return orDefault(s.AttributeFullName, attrFullNameDefault) }
func (s SAMLConfig) attrMail() string     { return orDefault(s.AttributeMail, attrMailDefault) }

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func loadOrCreateConfig() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefaultConfig()
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Backfill anything missing from older config files.
	changed := false
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = generateRandomPassword(16)
		fmt.Printf("Generated new admin password: %s\n", cfg.AdminPassword)
		changed = true
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8096"
		changed = true
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "data"
		changed = true
	}
	if changed {
		if err := saveConfig(&cfg); err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

func createDefaultConfig() (*Config, error) {
	password := generateRandomPassword(16)

	cfg := &Config{
		ListenAddr:    ":8096",
		AdminPassword: password,
		DataDir:       "data",
		SAML: SAMLConfig{
			Enabled:                    false,
			AllowLocalPasswordFallback: true,
			NameIDFormat:               "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
		},
	}

	if err := saveConfig(cfg); err != nil {
		return nil, err
	}

	fmt.Printf("Created %s with generated admin password: %s\n", configFile, password)
	fmt.Println("IMPORTANT: Log in as 'admin' with this password to start the setup wizard.")
	return cfg, nil
}

func saveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(configFile, data, 0600)
}

// dataPath returns an absolute path inside the configured data directory.
func (cfg *Config) dataPath(elem ...string) string {
	return filepath.Join(append([]string{cfg.DataDir}, elem...)...)
}

func generateRandomPassword(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic("failed to read random bytes: " + err.Error())
	}
	return hex.EncodeToString(b)[:length]
}
