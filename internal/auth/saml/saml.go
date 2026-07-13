// Package saml implements the SAML 2.0 SP engine: flow state with replay
// protection, response signature validation, IdP metadata parsing and the
// in-memory debug/capture stores surfaced by the admin SSO subtab. The HTTP
// handlers live in the web layer; this package is transport-agnostic except
// for BaseURL, which derives the external URL from proxy headers.
package saml

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/russellhaering/goxmldsig/etreeutils"
)

// Legacy SimpleSAMLphp paths. The ACS (reply) URL must stay identical to the
// old deployment so the existing Entra app registration needs no changes.
const (
	ACSPath    = "/simplesaml/module.php/saml/sp/saml2-acs.php/default-sp"
	LogoutPath = "/simplesaml/module.php/saml/sp/saml2-logout.php/default-sp"
)

// --- SAML flow state (pending + replay protection) ---

type FlowStore struct {
	mu              sync.Mutex
	pendingRequests map[string]time.Time
	consumedIDs     map[string]time.Time
}

var Flows = &FlowStore{
	pendingRequests: make(map[string]time.Time),
	consumedIDs:     make(map[string]time.Time),
}

func (s *FlowStore) Add(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	s.pendingRequests[id] = time.Now().Add(10 * time.Minute)
}

func (s *FlowStore) Consume(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.pendingRequests[id]
	if !ok || time.Now().After(exp) {
		return false
	}
	delete(s.pendingRequests, id)
	return true
}

func (s *FlowStore) IsConsumed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.consumedIDs[id]
	return ok && time.Now().Before(exp)
}

func (s *FlowStore) MarkConsumed(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consumedIDs[id] = time.Now().Add(10 * time.Minute)
}

func (s *FlowStore) cleanup() {
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

type DebugInfo struct {
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

type DebugStoreType struct {
	mu      sync.RWMutex
	pending map[string]*DebugInfo
	last    *DebugInfo
}

var DebugStore = &DebugStoreType{pending: make(map[string]*DebugInfo)}

func (s *DebugStoreType) Start(requestID string, info *DebugInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[requestID] = info
	s.last = info
}

func (s *DebugStoreType) Finish(requestID string, info *DebugInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, requestID)
	s.last = info
}

func (s *DebugStoreType) GetLast() *DebugInfo {
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

type Capture struct {
	When       string            `json:"when"`
	Username   string            `json:"username"`
	RawXML     string            `json:"raw_xml"`
	Attributes map[string]string `json:"attributes"`
}

var captures = struct {
	mu sync.Mutex
	m  map[string]*Capture
}{m: make(map[string]*Capture)}

func StoreCapture(c *Capture) {
	captures.mu.Lock()
	captures.m[strings.ToLower(c.Username)] = c
	captures.mu.Unlock()
}

func GetCapture(username string) *Capture {
	captures.mu.Lock()
	defer captures.mu.Unlock()
	return captures.m[strings.ToLower(username)]
}

func BaseURL(r *http.Request) string {
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

// --- IdP metadata parser ---

type IdPMetadata struct {
	SSOURL      string
	Certificate string
	EntityID    string
	TenantID    string
}

func ParseIdPMetadata(data []byte) (*IdPMetadata, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, fmt.Errorf("XML parse error: %w", err)
	}
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("empty XML document")
	}
	result := &IdPMetadata{EntityID: root.SelectAttrValue("entityID", "")}

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

func NowRFC() string { return time.Now().UTC().Format(time.RFC3339) }

func GenerateID() string {
	b := make([]byte, 20)
	rand.Read(b)
	return "_" + hex.EncodeToString(b)
}

func DeflateAndEncode(data []byte) (string, error) {
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

func BuildAuthnRequest(id, issueInstant, issuer, destination, acsURL, nameIDFormat string) string {
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

func BuildCertStore(certBase64 string) (*dsig.MemoryX509CertificateStore, error) {
	cert, err := ParseCertBase64(certBase64)
	if err != nil {
		return nil, err
	}
	return &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}, nil
}

func ValidateElement(ctx *dsig.ValidationContext, el *etree.Element) (*etree.Element, error) {
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

func CertFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

func ParseCertBase64(certBase64 string) (*x509.Certificate, error) {
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

func ExtractResponseCert(doc *etree.Document) (*x509.Certificate, string) {
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
		if cert, err := ParseCertBase64(raw); err == nil {
			return cert, raw
		}
	}
	return nil, ""
}

func ExtractAttr(assertion *etree.Element, attrName string) string {
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

func ExtractAllAttrs(assertion *etree.Element) map[string]string {
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
