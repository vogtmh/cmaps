package main

// SAML SSO HTTP handlers. The SP engine (flow state, signature validation,
// metadata parsing, debug/capture stores) lives in internal/auth/saml; these
// handlers wire it to the session store, user records and config.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"companymaps/internal/auth/saml"
	"companymaps/internal/config"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

func (app *App) samlEntityID(r *http.Request) string {
	if app.cfg.SAML.AppEntityID != "" {
		return app.cfg.SAML.AppEntityID
	}
	return saml.BaseURL(r)
}

func (app *App) samlACSURL(r *http.Request) string {
	if app.cfg.SAML.AppReplyURL != "" {
		return app.cfg.SAML.AppReplyURL
	}
	return saml.BaseURL(r) + saml.ACSPath
}

// --- Handlers ---

func (app *App) handleSAMLMetadata(w http.ResponseWriter, r *http.Request) {
	entityID := app.samlEntityID(r)
	acsURL := app.samlACSURL(r)
	logoutURL := app.cfg.SAML.AppLogoutURL
	if logoutURL == "" {
		logoutURL = saml.BaseURL(r) + saml.LogoutPath
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

	requestID := saml.GenerateID()
	issueInstant := time.Now().UTC().Format(time.RFC3339)
	authnReq := saml.BuildAuthnRequest(requestID, issueInstant, entityID, cfg.EntraLoginURL, acsURL, cfg.NameIDFormat)
	encoded, err := saml.DeflateAndEncode([]byte(authnReq))
	if err != nil {
		log.Printf("SAML login: encode AuthnRequest: %v", err)
		http.Error(w, "failed to initiate SAML login", http.StatusInternalServerError)
		return
	}

	saml.Flows.Add(requestID)

	debugMode := r.URL.Query().Get("debug") == "1"
	if debugMode {
		saml.DebugStore.Start(requestID, &saml.DebugInfo{
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

	certStore, err := saml.BuildCertStore(cfg.EntraX509Certificate)
	if err != nil {
		log.Printf("SAML ACS: cert store: %v", err)
		fail("Server configuration error (invalid X.509 certificate).")
		return
	}

	validationCtx := dsig.NewDefaultValidationContext(certStore)
	if confCert, cerr := saml.ParseCertBase64(cfg.EntraX509Certificate); cerr == nil {
		anchor := confCert.NotBefore.Add(confCert.NotAfter.Sub(confCert.NotBefore) / 2)
		validationCtx.Clock = dsig.NewFakeClockAt(anchor)
	} else {
		validationCtx.Clock = dsig.NewRealClock()
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		if debugRequestID != "" {
			saml.DebugStore.Finish(debugRequestID, &saml.DebugInfo{Status: "error", Message: "Malformed SAML response (XML parse failed).", CompletedAt: saml.NowRFC()})
			debugDone("Parse failed — check debug info in settings.")
			return
		}
		fail("Malformed SAML response.")
		return
	}

	root := doc.Root()
	assertionEl := root.FindElement(".//Assertion")

	certDiag := &saml.DebugInfo{}
	if respCert, respRaw := saml.ExtractResponseCert(doc); respCert != nil {
		certDiag.ResponseCertSubject = respCert.Subject.CommonName
		certDiag.ResponseCertFingerprint = saml.CertFingerprint(respCert)
		certDiag.ResponseCertNotAfter = respCert.NotAfter.UTC().Format(time.RFC3339)
		certDiag.ResponseCertBase64 = respRaw
		if confCert, err := saml.ParseCertBase64(cfg.EntraX509Certificate); err == nil {
			certDiag.ConfiguredCertSubject = confCert.Subject.CommonName
			certDiag.ConfiguredCertFingerprint = saml.CertFingerprint(confCert)
			certDiag.CertMatch = saml.CertFingerprint(confCert) == certDiag.ResponseCertFingerprint
		}
	}
	applyCertDiag := func(info *saml.DebugInfo) *saml.DebugInfo {
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
		validatedEl, validationErr = saml.ValidateElement(validationCtx, assertionEl)
		if validationErr != nil {
			assertionErr := validationErr
			var rootValidated *etree.Element
			rootValidated, validationErr = saml.ValidateElement(validationCtx, root)
			if validationErr != nil {
				log.Printf("SAML ACS: assertion signature: %v; response signature: %v", assertionErr, validationErr)
				validationErr = fmt.Errorf("assertion signature invalid (%v)", assertionErr)
			} else {
				validatedEl = rootValidated
			}
		}
	} else {
		validatedEl, validationErr = saml.ValidateElement(validationCtx, root)
	}
	if validationErr != nil {
		log.Printf("SAML ACS: signature validation: %v", validationErr)
		if debugRequestID != "" {
			msg := "SAML signature validation failed: " + validationErr.Error()
			if certDiag.ResponseCertFingerprint != "" && !certDiag.CertMatch {
				msg += " — the configured certificate does not match the certificate the IdP signed with. " +
					"Copy response_cert_base64 below into the X.509 certificate setting."
			}
			saml.DebugStore.Finish(debugRequestID, applyCertDiag(&saml.DebugInfo{Status: "error", Message: msg, CompletedAt: saml.NowRFC()}))
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
	if saml.Flows.IsConsumed(responseID) || saml.Flows.IsConsumed(assertionID) {
		fail("This SAML response was already used.")
		return
	}

	inResponseTo := root.SelectAttrValue("InResponseTo", "")
	if inResponseTo == "" {
		if sc := assertion.FindElement(".//SubjectConfirmationData"); sc != nil {
			inResponseTo = sc.SelectAttrValue("InResponseTo", "")
		}
	}
	if inResponseTo != "" && !saml.Flows.Consume(inResponseTo) {
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

	saml.Flows.MarkConsumed(responseID)
	saml.Flows.MarkConsumed(assertionID)

	samaccountname := saml.ExtractAttr(assertion, cfg.AttrSamAccount())
	givenname := saml.ExtractAttr(assertion, cfg.AttrGivenName())
	surname := saml.ExtractAttr(assertion, cfg.AttrSurname())
	fullname := saml.ExtractAttr(assertion, cfg.AttrFullName())
	mail := normalizeMail(saml.ExtractAttr(assertion, cfg.AttrMail()))
	if fullname == "" {
		fullname = strings.TrimSpace(givenname + " " + surname)
	}
	if fullname == "" {
		fullname = orDefault(mail, samaccountname)
	}

	if debugRequestID != "" {
		saml.DebugStore.Finish(debugRequestID, applyCertDiag(&saml.DebugInfo{
			Status:             "success",
			Message:            "The SAML response was accepted. Return to the settings page and refresh the debug info.",
			CompletedAt:        saml.NowRFC(),
			SignatureValidated: true,
			UserEmail:          mail,
			DisplayName:        fullname,
			Attributes:         saml.ExtractAllAttrs(assertion),
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
		saml.StoreCapture(&saml.Capture{
			When:       time.Now().In(app.db.Location()).Format("2006-01-02 15:04:05"),
			Username:   username,
			RawXML:     string(xmlBytes),
			Attributes: saml.ExtractAllAttrs(assertion),
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
	} else if cert, err := saml.ParseCertBase64(cfg.EntraX509Certificate); err != nil {
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
		reqXML := saml.BuildAuthnRequest(saml.GenerateID(), time.Now().UTC().Format(time.RFC3339), entityID, loginURL, acsURL, cfg.NameIDFormat)
		if _, err := saml.DeflateAndEncode([]byte(reqXML)); err != nil {
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
		"sign_on_url":  saml.BaseURL(r) + "/",
		"metadata_url": saml.BaseURL(r) + "/auth/saml/metadata",
		"login_url":    saml.BaseURL(r) + "/auth/saml/login",
	})
}

func (app *App) handleSAMLDebugLast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info := saml.DebugStore.GetLast()
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
	c := saml.GetCapture(sess.Username)
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
	parsed, err := saml.ParseIdPMetadata(body)
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
