package main

import (
	"bytes"
	"companymaps/internal/config"
	"compress/flate"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/russellhaering/goxmldsig/etreeutils"
)

// Legacy SimpleSAMLphp paths. The ACS (reply) URL must stay identical to the old
// deployment so the existing Entra app registration needs no changes.
const (
	samlACSPath    = "/simplesaml/module.php/saml/sp/saml2-acs.php/default-sp"
	samlLogoutPath = "/simplesaml/module.php/saml/sp/saml2-logout.php/default-sp"
)

// --- SAML flow state (pending + replay protection) ---

type samlFlowStore struct {
	mu              sync.Mutex
	pendingRequests map[string]time.Time
	consumedIDs     map[string]time.Time
}

var samlFlows = &samlFlowStore{
	pendingRequests: make(map[string]time.Time),
	consumedIDs:     make(map[string]time.Time),
}

func (s *samlFlowStore) add(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	s.pendingRequests[id] = time.Now().Add(10 * time.Minute)
}

func (s *samlFlowStore) consume(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.pendingRequests[id]
	if !ok || time.Now().After(exp) {
		return false
	}
	delete(s.pendingRequests, id)
	return true
}

func (s *samlFlowStore) isConsumed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.consumedIDs[id]
	return ok && time.Now().Before(exp)
}

func (s *samlFlowStore) markConsumed(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consumedIDs[id] = time.Now().Add(10 * time.Minute)
}

func (s *samlFlowStore) cleanup() {
	now := time.Now()
	for id, exp := range s.pendingRequests {
		if now.After(exp) {
			delete(s.pendingRequests, id)
		}
	}
	for id, exp := range s.consumedIDs {
		if now.After(exp) {
			delete(s.consumedIDs, id)
		}
	}
}

// --- Debug store (for the "Test SAML" button on the settings page) ---

type samlDebugInfo struct {
	Status             string            `json:"status"`
	Message            string            `json:"message"`
	StartedAt          string            `json:"started_at,omitempty"`
	CompletedAt        string            `json:"completed_at,omitempty"`
	RequestID          string            `json:"request_id,omitempty"`
	SignatureValidated bool              `json:"signature_validated"`
	UserEmail          string            `json:"user_email,omitempty"`
	DisplayName        string            `json:"display_name,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`

	ResponseCertSubject       string `json:"response_cert_subject,omitempty"`
	ResponseCertFingerprint   string `json:"response_cert_fingerprint,omitempty"`
	ResponseCertNotAfter      string `json:"response_cert_not_after,omitempty"`
	ResponseCertBase64        string `json:"response_cert_base64,omitempty"`
	ConfiguredCertSubject     string `json:"configured_cert_subject,omitempty"`
	ConfiguredCertFingerprint string `json:"configured_cert_fingerprint,omitempty"`
	CertMatch                 bool   `json:"cert_match"`
}

type samlDebugStoreType struct {
	mu      sync.RWMutex
	pending map[string]*samlDebugInfo
	last    *samlDebugInfo
}

var samlDebugStore = &samlDebugStoreType{pending: make(map[string]*samlDebugInfo)}

func (s *samlDebugStoreType) start(requestID string, info *samlDebugInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[requestID] = info
	s.last = info
}

func (s *samlDebugStoreType) finish(requestID string, info *samlDebugInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, requestID)
	s.last = info
}

func (s *samlDebugStoreType) getLast() *samlDebugInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last
}

// --- Captured live login responses (for the "Show my SAML login response"
// button on the SAML SSO subtab) ---
//
// On every successful SAML login by a user who holds an admin role, we keep the
// full decoded SAML response XML and its parsed attributes in memory, keyed by
// username. An admin can then inspect exactly what their IdP sent (which claims,
// which mail value, etc.) without wiring up a separate debug login. It is a
// diagnostic aid only, held in memory and cleared on restart.

type samlCapture struct {
	When       string            `json:"when"`
	Username   string            `json:"username"`
	RawXML     string            `json:"raw_xml"`
	Attributes map[string]string `json:"attributes"`
}

var samlCaptures = struct {
	mu sync.Mutex
	m  map[string]*samlCapture
}{m: make(map[string]*samlCapture)}

func samlStoreCapture(c *samlCapture) {
	samlCaptures.mu.Lock()
	samlCaptures.m[strings.ToLower(c.Username)] = c
	samlCaptures.mu.Unlock()
}

func samlGetCapture(username string) *samlCapture {
	samlCaptures.mu.Lock()
	defer samlCaptures.mu.Unlock()
	return samlCaptures.m[strings.ToLower(username)]
}

func samlBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		} else {
			scheme = "http"
		}
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	return scheme + "://" + host
}

func (app *App) samlEntityID(r *http.Request) string {
	if app.cfg.SAML.AppEntityID != "" {
		return app.cfg.SAML.AppEntityID
	}
	return samlBaseURL(r)
}

func (app *App) samlACSURL(r *http.Request) string {
	if app.cfg.SAML.AppReplyURL != "" {
		return app.cfg.SAML.AppReplyURL
	}
	return samlBaseURL(r) + samlACSPath
}

// --- Handlers ---

func (app *App) handleSAMLMetadata(w http.ResponseWriter, r *http.Request) {
	entityID := app.samlEntityID(r)
	acsURL := app.samlACSURL(r)
	logoutURL := app.cfg.SAML.AppLogoutURL
	if logoutURL == "" {
		logoutURL = samlBaseURL(r) + samlLogoutPath
	}

	metadata := `<?xml version="1.0" encoding="UTF-8"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="` + entityID + `">
  <SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
    <NameIDFormat>` + orDefault(app.cfg.SAML.NameIDFormat, "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent") + `</NameIDFormat>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="` + acsURL + `" isDefault="true" index="0"/>
    <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="` + logoutURL + `"/>
  </SPSSODescriptor>
</EntityDescriptor>`

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(metadata))
}

func (app *App) handleSAMLLogin(w http.ResponseWriter, r *http.Request) {
	cfg := app.cfg.SAML
	if !cfg.Enabled {
		http.Error(w, "SAML is not enabled", http.StatusBadRequest)
		return
	}
	if cfg.EntraLoginURL == "" {
		http.Error(w, "SSO URL not configured", http.StatusInternalServerError)
		return
	}

	entityID := app.samlEntityID(r)
	acsURL := app.samlACSURL(r)

	requestID := samlGenerateID()
	issueInstant := time.Now().UTC().Format(time.RFC3339)
	authnReq := buildSAMLAuthnRequest(requestID, issueInstant, entityID, cfg.EntraLoginURL, acsURL, cfg.NameIDFormat)
	encoded, err := samlDeflateAndEncode([]byte(authnReq))
	if err != nil {
		log.Printf("SAML login: encode AuthnRequest: %v", err)
		http.Error(w, "failed to initiate SAML login", http.StatusInternalServerError)
		return
	}

	samlFlows.add(requestID)

	debugMode := r.URL.Query().Get("debug") == "1"
	if debugMode {
		samlDebugStore.start(requestID, &samlDebugInfo{
			Status:    "started",
			Message:   "Live SAML test started. Waiting for SAML response.",
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			RequestID: requestID,
		})
	}

	redirectURL, err := url.Parse(cfg.EntraLoginURL)
	if err != nil {
		http.Error(w, "invalid SSO URL", http.StatusInternalServerError)
		return
	}
	q := redirectURL.Query()
	q.Set("SAMLRequest", encoded)
	if debugMode {
		q.Set("RelayState", "debug:"+requestID)
	} else if next := safeNextPath(r.URL.Query().Get("next")); next != "/" {
		// Carry the originally requested page through the IdP round-trip so the
		// ACS handler can return the user there after a successful login.
		q.Set("RelayState", next)
	}
	redirectURL.RawQuery = q.Encode()
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

func (app *App) handleSAMLLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		app.sessions.Delete(c.Value)
	}
	app.clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (app *App) handleSAMLACS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fail := func(msg string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Login failed</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#222}
.box{background:#fff;border:1px solid #e74c3c;border-radius:8px;padding:40px;max-width:480px;color:#333;text-align:center}
h2{color:#e74c3c;margin:0 0 16px}p{color:#666;margin:0 0 24px}a{color:#0a66c2}</style></head>
<body><div class="box"><h2>Authentication failed</h2><p>%s</p><a href="/">Back to CompanyMaps</a></div></body></html>`, msg)
	}

	debugDone := func(msg string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>SAML live test complete</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#222}
.box{background:#fff;border:1px solid #2ecc71;border-radius:8px;padding:40px;max-width:520px;color:#333;text-align:center}
h2{color:#2ecc71;margin:0 0 16px}p{color:#666;margin:0 0 24px}a{color:#0a66c2}</style></head>
<body><div class="box"><h2>SAML live test completed</h2><p>%s</p><a href="/admin?tab=saml">Return to settings</a></div></body></html>`, msg)
	}

	r.ParseForm()
	rawResponse := r.FormValue("SAMLResponse")
	relayState := r.FormValue("RelayState")
	debugRequestID := ""
	returnPath := "/"
	if strings.HasPrefix(relayState, "debug:") {
		debugRequestID = strings.TrimPrefix(relayState, "debug:")
	} else {
		// A non-debug RelayState carries the page the user originally requested.
		returnPath = safeNextPath(relayState)
	}

	if rawResponse == "" {
		fail("No SAML response received.")
		return
	}

	cfg := app.cfg.SAML
	if !cfg.Enabled {
		fail("SAML authentication is not enabled.")
		return
	}
	if cfg.EntraX509Certificate == "" {
		fail("SAML is not fully configured (missing X.509 certificate).")
		return
	}

	xmlBytes, err := base64.StdEncoding.DecodeString(rawResponse)
	if err != nil {
		fail("Malformed SAML response.")
		return
	}

	certStore, err := samlBuildCertStore(cfg.EntraX509Certificate)
	if err != nil {
		log.Printf("SAML ACS: cert store: %v", err)
		fail("Server configuration error (invalid X.509 certificate).")
		return
	}

	validationCtx := dsig.NewDefaultValidationContext(certStore)
	if confCert, cerr := samlParseCertBase64(cfg.EntraX509Certificate); cerr == nil {
		anchor := confCert.NotBefore.Add(confCert.NotAfter.Sub(confCert.NotBefore) / 2)
		validationCtx.Clock = dsig.NewFakeClockAt(anchor)
	} else {
		validationCtx.Clock = dsig.NewRealClock()
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		if debugRequestID != "" {
			samlDebugStore.finish(debugRequestID, &samlDebugInfo{Status: "error", Message: "Malformed SAML response (XML parse failed).", CompletedAt: nowRFC()})
			debugDone("Parse failed — check debug info in settings.")
			return
		}
		fail("Malformed SAML response.")
		return
	}

	root := doc.Root()
	assertionEl := root.FindElement(".//Assertion")

	certDiag := &samlDebugInfo{}
	if respCert, respRaw := samlExtractResponseCert(doc); respCert != nil {
		certDiag.ResponseCertSubject = respCert.Subject.CommonName
		certDiag.ResponseCertFingerprint = samlCertFingerprint(respCert)
		certDiag.ResponseCertNotAfter = respCert.NotAfter.UTC().Format(time.RFC3339)
		certDiag.ResponseCertBase64 = respRaw
		if confCert, err := samlParseCertBase64(cfg.EntraX509Certificate); err == nil {
			certDiag.ConfiguredCertSubject = confCert.Subject.CommonName
			certDiag.ConfiguredCertFingerprint = samlCertFingerprint(confCert)
			certDiag.CertMatch = samlCertFingerprint(confCert) == certDiag.ResponseCertFingerprint
		}
	}
	applyCertDiag := func(info *samlDebugInfo) *samlDebugInfo {
		info.ResponseCertSubject = certDiag.ResponseCertSubject
		info.ResponseCertFingerprint = certDiag.ResponseCertFingerprint
		info.ResponseCertNotAfter = certDiag.ResponseCertNotAfter
		info.ResponseCertBase64 = certDiag.ResponseCertBase64
		info.ConfiguredCertSubject = certDiag.ConfiguredCertSubject
		info.ConfiguredCertFingerprint = certDiag.ConfiguredCertFingerprint
		info.CertMatch = certDiag.CertMatch
		return info
	}

	var validatedEl *etree.Element
	var validationErr error
	if assertionEl != nil {
		validatedEl, validationErr = samlValidateElement(validationCtx, assertionEl)
		if validationErr != nil {
			assertionErr := validationErr
			var rootValidated *etree.Element
			rootValidated, validationErr = samlValidateElement(validationCtx, root)
			if validationErr != nil {
				log.Printf("SAML ACS: assertion signature: %v; response signature: %v", assertionErr, validationErr)
				validationErr = fmt.Errorf("assertion signature invalid (%v)", assertionErr)
			} else {
				validatedEl = rootValidated
			}
		}
	} else {
		validatedEl, validationErr = samlValidateElement(validationCtx, root)
	}
	if validationErr != nil {
		log.Printf("SAML ACS: signature validation: %v", validationErr)
		if debugRequestID != "" {
			msg := "SAML signature validation failed: " + validationErr.Error()
			if certDiag.ResponseCertFingerprint != "" && !certDiag.CertMatch {
				msg += " — the configured certificate does not match the certificate the IdP signed with. " +
					"Copy response_cert_base64 below into the X.509 certificate setting."
			}
			samlDebugStore.finish(debugRequestID, applyCertDiag(&samlDebugInfo{Status: "error", Message: msg, CompletedAt: nowRFC()}))
			debugDone("Signature validation failed — check debug info in settings.")
			return
		}
		fail("SAML signature validation failed.")
		return
	}

	assertion := validatedEl
	if validatedEl.Tag != "Assertion" && !strings.HasSuffix(validatedEl.Tag, ":Assertion") {
		assertion = validatedEl.FindElement(".//Assertion")
		if assertion == nil {
			fail("SAML response contains no Assertion.")
			return
		}
	}

	responseID := root.SelectAttrValue("ID", "")
	assertionID := assertion.SelectAttrValue("ID", "")
	if samlFlows.isConsumed(responseID) || samlFlows.isConsumed(assertionID) {
		fail("This SAML response was already used.")
		return
	}

	inResponseTo := root.SelectAttrValue("InResponseTo", "")
	if inResponseTo == "" {
		if sc := assertion.FindElement(".//SubjectConfirmationData"); sc != nil {
			inResponseTo = sc.SelectAttrValue("InResponseTo", "")
		}
	}
	if inResponseTo != "" && !samlFlows.consume(inResponseTo) {
		fail("SAML response does not match an active login request. Please try again.")
		return
	}

	now := time.Now().UTC()
	if conds := assertion.FindElement(".//Conditions"); conds != nil {
		if nb := conds.SelectAttrValue("NotBefore", ""); nb != "" {
			if t, err := time.Parse(time.RFC3339, nb); err == nil && now.Before(t.Add(-30*time.Second)) {
				fail("SAML assertion is not yet valid.")
				return
			}
		}
		if na := conds.SelectAttrValue("NotOnOrAfter", ""); na != "" {
			if t, err := time.Parse(time.RFC3339, na); err == nil && !now.Before(t.Add(30*time.Second)) {
				fail("SAML assertion has expired.")
				return
			}
		}
	}

	if cfg.AppEntityID != "" {
		audiences := assertion.FindElements(".//AudienceRestriction/Audience")
		matched := len(audiences) == 0
		for _, a := range audiences {
			if strings.TrimSpace(a.Text()) == cfg.AppEntityID {
				matched = true
				break
			}
		}
		if !matched {
			fail("SAML audience restriction does not match this application.")
			return
		}
	}

	samlFlows.markConsumed(responseID)
	samlFlows.markConsumed(assertionID)

	samaccountname := samlExtractAttr(assertion, cfg.AttrSamAccount())
	givenname := samlExtractAttr(assertion, cfg.AttrGivenName())
	surname := samlExtractAttr(assertion, cfg.AttrSurname())
	fullname := samlExtractAttr(assertion, cfg.AttrFullName())
	mail := normalizeMail(samlExtractAttr(assertion, cfg.AttrMail()))
	if fullname == "" {
		fullname = strings.TrimSpace(givenname + " " + surname)
	}
	if fullname == "" {
		fullname = orDefault(mail, samaccountname)
	}

	if debugRequestID != "" {
		samlDebugStore.finish(debugRequestID, applyCertDiag(&samlDebugInfo{
			Status:             "success",
			Message:            "The SAML response was accepted. Return to the settings page and refresh the debug info.",
			CompletedAt:        nowRFC(),
			SignatureValidated: true,
			UserEmail:          mail,
			DisplayName:        fullname,
			Attributes:         samlExtractAllAttrs(assertion),
		}))
		debugDone("The SAML response was accepted. Return to the settings page and refresh the debug info.")
		return
	}

	// In samaccountname mode the SamAccountName claim IS the identity, so it is
	// mandatory. In mail mode the identity comes from the mail address, so a
	// SamAccountName claim is not required — the IdP may stop emitting it once the
	// tenant has moved off on-prem AD, and SSO must keep working regardless.
	if app.identifierMode() != "mail" && samaccountname == "" {
		fail("SAML assertion did not contain a SamAccountName attribute.")
		return
	}

	// Compute the active identifier (samaccountname or mail-based) and the
	// mapadmins-style username so the existing RBAC records match. In mail mode
	// the username is the bare mail identifier (no DOMAIN\ prefix, which only
	// applies to samaccountname logins). Persist the user so admins can grant a
	// role later.
	identifier := app.userIdentifier(samaccountname, mail)
	if identifier == "" {
		fail("SAML login uses the e-mail address as the identifier, but the assertion did not contain a mail attribute.")
		return
	}
	username := identifier
	if app.identifierMode() != "mail" {
		if domain := app.db.GetSetting("domain"); domain != "" {
			username = domain + "\\" + samaccountname
		}
	}
	u, known, _ := app.db.GetUser(username)
	if !known {
		u = User{Username: username, Role: 0}
		_ = app.db.AuditLog("users", username, "New SAML user registered: "+username+" ("+fullname+")")
	}
	u.Fullname = fullname
	u.Mail = mail
	u.LastLogin = time.Now().In(app.db.Location()).Format("2006-01-02 15:04:05")
	_ = app.db.PutUser(u)

	// Capture the full response for admins so they can inspect exactly what the
	// IdP sent from the SAML SSO subtab.
	if u.Role != 0 {
		samlStoreCapture(&samlCapture{
			When:       time.Now().In(app.db.Location()).Format("2006-01-02 15:04:05"),
			Username:   username,
			RawXML:     string(xmlBytes),
			Attributes: samlExtractAllAttrs(assertion),
		})
	}

	sess := Session{
		Samaccountname: identifier,
		Username:       username,
		Fullname:       fullname,
		Mail:           mail,
	}
	token, err := app.sessions.Create(sess)
	if err != nil {
		fail("Internal error creating session.")
		return
	}
	_ = app.db.AuditLog("account", username, fullname+" has been logged in via SAML")
	log.Printf("SAML ACS: successful login for %s (%s)", username, fullname)

	app.setSessionCookie(w, token)
	app.resetUsermodeCookie(w)
	http.Redirect(w, r, returnPath, http.StatusFound)
}

// --- SAML settings / status / debug REST endpoints (admin only) ---

func (app *App) handleSAMLStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := app.cfg.SAML
	configured := cfg.EntraLoginURL != "" && cfg.EntraX509Certificate != ""
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":    cfg.Enabled,
		"configured": configured,
	})
}

// handleSAMLValidate runs a server-side pre-flight check of the SAML
// configuration so the admin can verify it without the interactive IdP
// round-trip (no new browser window). It validates the certificate, confirms an
// AuthnRequest can be generated, and probes the IdP login endpoint reachability.
func (app *App) handleSAMLValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := app.cfg.SAML

	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"` // ok | warn | fail
		Detail string `json:"detail"`
	}
	var checks []check
	add := func(name, status, detail string) {
		checks = append(checks, check{Name: name, Status: status, Detail: detail})
	}

	// SAML enabled?
	if cfg.Enabled {
		add("SAML enabled", "ok", "Single sign-on is switched on.")
	} else {
		add("SAML enabled", "warn", "SAML is currently disabled; users cannot sign in with it yet.")
	}

	// IdP entity ID.
	if strings.TrimSpace(cfg.EntraEntityID) != "" {
		add("IdP Entity ID", "ok", cfg.EntraEntityID)
	} else {
		add("IdP Entity ID", "warn", "No IdP entity ID set.")
	}

	// Login URL present + parseable.
	loginURL := strings.TrimSpace(cfg.EntraLoginURL)
	var parsedLogin *url.URL
	if loginURL == "" {
		add("Login URL", "fail", "No SSO login URL configured.")
	} else if u, err := url.Parse(loginURL); err != nil || u.Scheme == "" || u.Host == "" {
		add("Login URL", "fail", "Login URL is not a valid absolute URL.")
	} else {
		parsedLogin = u
		add("Login URL", "ok", loginURL)
	}

	// Certificate parses + expiry.
	if strings.TrimSpace(cfg.EntraX509Certificate) == "" {
		add("X.509 certificate", "fail", "No signing certificate configured.")
	} else if cert, err := samlParseCertBase64(cfg.EntraX509Certificate); err != nil {
		add("X.509 certificate", "fail", "Certificate could not be parsed: "+err.Error())
	} else {
		now := time.Now()
		detail := "Subject: " + cert.Subject.CommonName + " · expires " + cert.NotAfter.Format("2006-01-02")
		switch {
		case now.After(cert.NotAfter):
			add("X.509 certificate", "fail", "Certificate expired on "+cert.NotAfter.Format("2006-01-02")+".")
		case now.Before(cert.NotBefore):
			add("X.509 certificate", "warn", "Certificate is not valid until "+cert.NotBefore.Format("2006-01-02")+".")
		case now.Add(30 * 24 * time.Hour).After(cert.NotAfter):
			add("X.509 certificate", "warn", "Certificate expires soon — "+detail)
		default:
			add("X.509 certificate", "ok", detail)
		}
	}

	// AuthnRequest generation.
	if parsedLogin != nil {
		entityID := app.samlEntityID(r)
		acsURL := app.samlACSURL(r)
		reqXML := buildSAMLAuthnRequest(samlGenerateID(), time.Now().UTC().Format(time.RFC3339), entityID, loginURL, acsURL, cfg.NameIDFormat)
		if _, err := samlDeflateAndEncode([]byte(reqXML)); err != nil {
			add("AuthnRequest", "fail", "Failed to build the SAML request: "+err.Error())
		} else {
			add("AuthnRequest", "ok", "A valid SAML authentication request can be generated.")
		}

		// Reachability probe of the IdP login endpoint (any HTTP response counts
		// as reachable; we do not follow the SSO flow here).
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Get(loginURL)
		if err != nil {
			add("IdP reachable", "warn", "Could not reach the login URL: "+err.Error())
		} else {
			resp.Body.Close()
			add("IdP reachable", "ok", fmt.Sprintf("Login endpoint responded (HTTP %d).", resp.StatusCode))
		}
	}

	ok := true
	for _, c := range checks {
		if c.Status == "fail" {
			ok = false
			break
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": ok, "checks": checks})
}

func (app *App) handleSAMLSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(app.cfg.SAML)
	case http.MethodPut, http.MethodPost:
		var incoming config.SAMLConfig
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		app.cfg.SAML = incoming
		if err := config.Save(app.cfg); err != nil {
			log.Printf("SAML settings: save: %v", err)
			http.Error(w, "failed to save settings", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (app *App) handleSAMLSPInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"entity_id":    app.samlEntityID(r),
		"acs_url":      app.samlACSURL(r),
		"sign_on_url":  samlBaseURL(r) + "/",
		"metadata_url": samlBaseURL(r) + "/auth/saml/metadata",
		"login_url":    samlBaseURL(r) + "/auth/saml/login",
	})
}

func (app *App) handleSAMLDebugLast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info := samlDebugStore.getLast()
	if info == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "none", "message": "No live test has been run yet."})
		return
	}
	json.NewEncoder(w).Encode(info)
}

// handleSAMLMyCapture returns the full SAML response captured for the currently
// logged-in admin on their most recent SAML login (raw XML + parsed attributes).
func (app *App) handleSAMLMyCapture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sess, ok := app.currentSession(r)
	if !ok {
		json.NewEncoder(w).Encode(map[string]string{"status": "none", "message": "Not signed in."})
		return
	}
	c := samlGetCapture(sess.Username)
	if c == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "none",
			"message": "No SAML response captured for your account yet. Sign in via SAML SSO (not local login) to capture one."})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"when":       c.When,
		"username":   c.Username,
		"attributes": c.Attributes,
		"raw_xml":    c.RawXML,
	})
}

func (app *App) handleSAMLImportMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "url required"})
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(req.URL)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fetch failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "read failed: " + err.Error()})
		return
	}
	parsed, err := parseIdPMetadata(body)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "parse failed: " + err.Error()})
		return
	}
	app.cfg.SAML.EntraLoginURL = parsed.SSOURL
	app.cfg.SAML.EntraX509Certificate = parsed.Certificate
	app.cfg.SAML.EntraEntityID = parsed.EntityID
	if parsed.TenantID != "" {
		app.cfg.SAML.EntraTenantID = parsed.TenantID
	}
	app.cfg.SAML.FederationMetadataURL = req.URL
	if err := config.Save(app.cfg); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "save failed: " + err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"login_url": parsed.SSOURL,
		"x509_cert": parsed.Certificate,
		"entity_id": parsed.EntityID,
		"tenant_id": parsed.TenantID,
	})
}

// --- IdP metadata parser ---

type idPMetadata struct {
	SSOURL      string
	Certificate string
	EntityID    string
	TenantID    string
}

func parseIdPMetadata(data []byte) (*idPMetadata, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, fmt.Errorf("XML parse error: %w", err)
	}
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("empty XML document")
	}
	result := &idPMetadata{EntityID: root.SelectAttrValue("entityID", "")}

	var idpDesc *etree.Element
	for _, child := range root.ChildElements() {
		if localName(child.Tag) == "IDPSSODescriptor" {
			idpDesc = child
			break
		}
	}
	if idpDesc == nil {
		idpDesc = root.FindElement("//IDPSSODescriptor")
	}
	if idpDesc != nil {
		var walk func(*etree.Element) *etree.Element
		walk = func(el *etree.Element) *etree.Element {
			for _, ch := range el.ChildElements() {
				if localName(ch.Tag) == "X509Certificate" {
					return ch
				}
				if found := walk(ch); found != nil {
					return found
				}
			}
			return nil
		}
		for _, kd := range idpDesc.FindElements("KeyDescriptor") {
			use := kd.SelectAttrValue("use", "")
			certEl := walk(kd)
			if certEl == nil {
				continue
			}
			certVal := strings.TrimSpace(certEl.Text())
			if certVal != "" && (use == "signing" || result.Certificate == "") {
				result.Certificate = certVal
				if use == "signing" {
					break
				}
			}
		}
		postBinding := "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
		redirectBinding := "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
		for _, sso := range idpDesc.FindElements("SingleSignOnService") {
			binding := sso.SelectAttrValue("Binding", "")
			location := sso.SelectAttrValue("Location", "")
			if binding == postBinding || (result.SSOURL == "" && binding == redirectBinding) {
				result.SSOURL = location
				if binding == postBinding {
					break
				}
			}
		}
	}
	if result.SSOURL == "" {
		return nil, fmt.Errorf("no SSO URL found in metadata")
	}
	if result.EntityID != "" {
		if u, err := url.Parse(result.EntityID); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) > 0 && parts[0] != "" {
				result.TenantID = parts[0]
			}
		}
	}
	return result, nil
}

// --- crypto / XML helpers ---

func localName(tag string) string {
	if idx := strings.LastIndexAny(tag, ":}"); idx >= 0 {
		return tag[idx+1:]
	}
	return tag
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339) }

func samlGenerateID() string {
	b := make([]byte, 20)
	rand.Read(b)
	return "_" + hex.EncodeToString(b)
}

func samlDeflateAndEncode(data []byte) (string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func buildSAMLAuthnRequest(id, issueInstant, issuer, destination, acsURL, nameIDFormat string) string {
	req := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<AuthnRequest xmlns="urn:oasis:names:tc:SAML:2.0:protocol" ID="` + id +
		`" Version="2.0" IssueInstant="` + issueInstant +
		`" Destination="` + destination +
		`" ProtocolBinding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"` +
		` AssertionConsumerServiceURL="` + acsURL + `">` +
		`<Issuer xmlns="urn:oasis:names:tc:SAML:2.0:assertion">` + issuer + `</Issuer>`
	if nameIDFormat != "" {
		req += `<NameIDPolicy AllowCreate="true" Format="` + nameIDFormat + `"/>`
	} else {
		req += `<NameIDPolicy AllowCreate="true" Format="urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"/>`
	}
	req += `</AuthnRequest>`
	return req
}

func samlBuildCertStore(certBase64 string) (*dsig.MemoryX509CertificateStore, error) {
	cert, err := samlParseCertBase64(certBase64)
	if err != nil {
		return nil, err
	}
	return &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}, nil
}

func samlValidateElement(ctx *dsig.ValidationContext, el *etree.Element) (*etree.Element, error) {
	nsCtx, err := etreeutils.NSBuildParentContext(el)
	if err != nil {
		return nil, err
	}
	detached, err := etreeutils.NSDetatch(nsCtx, el)
	if err != nil {
		return nil, err
	}
	return ctx.Validate(detached)
}

func samlCertFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

func samlParseCertBase64(certBase64 string) (*x509.Certificate, error) {
	cleaned := strings.ReplaceAll(certBase64, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.TrimPrefix(cleaned, "-----BEGINCERTIFICATE-----")
	cleaned = strings.TrimSuffix(cleaned, "-----ENDCERTIFICATE-----")
	der, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

func samlExtractResponseCert(doc *etree.Document) (*x509.Certificate, string) {
	root := doc.Root()
	if root == nil {
		return nil, ""
	}
	var certEls []*etree.Element
	if assertion := root.FindElement(".//Assertion"); assertion != nil {
		certEls = assertion.FindElements(".//X509Certificate")
	}
	if len(certEls) == 0 {
		certEls = root.FindElements(".//X509Certificate")
	}
	for _, el := range certEls {
		raw := strings.Join(strings.Fields(el.Text()), "")
		if raw == "" {
			continue
		}
		if cert, err := samlParseCertBase64(raw); err == nil {
			return cert, raw
		}
	}
	return nil, ""
}

func samlExtractAttr(assertion *etree.Element, attrName string) string {
	for _, attrEl := range assertion.FindElements(".//Attribute") {
		if attrEl.SelectAttrValue("Name", "") != attrName {
			continue
		}
		for _, val := range attrEl.FindElements("AttributeValue") {
			if v := strings.TrimSpace(val.Text()); v != "" {
				return v
			}
		}
	}
	return ""
}

func samlExtractAllAttrs(assertion *etree.Element) map[string]string {
	attrs := make(map[string]string)
	for _, attrEl := range assertion.FindElements(".//Attribute") {
		name := attrEl.SelectAttrValue("Name", "")
		if name == "" {
			continue
		}
		for _, val := range attrEl.FindElements("AttributeValue") {
			if v := strings.TrimSpace(val.Text()); v != "" {
				attrs[name] = v
				break
			}
		}
	}
	return attrs
}
