// Package directory implements the employee-directory model shared by the
// LDAP, EntraID and Robin integrations: the common identifier scheme
// (samaccountname vs. mail), directory lookups and the changelog.
package directory

import (
	"strings"

	"companymaps/internal/store"
)

// The employee "common identifier" is either the samaccountname (default,
// only available via LDAP) or the e-mail address (available via LDAP, EntraID
// and SAML SSO). It is the key used for avatar filenames, the audit log,
// map-admin records, bookings and the changelog.
//
// So the identifier is always a valid filename and URL path segment on
// Windows, macOS and Linux, the mail-mode identifier is the lowercased mail
// address with every filesystem/URL-unsafe character (notably "@") replaced
// by "_", e.g. "john.doe@teamviewer.com" -> "john.doe_teamviewer.com". The
// raw mail address is always kept separately (the Mail field) for display and
// contact. samaccountname identifiers are used verbatim (they never contain
// unsafe characters).

// Mode returns the configured identifier mode, defaulting to "mail" for new
// installs. Installs created before this default changed are pinned to
// "samaccountname" at startup (see store pinLegacyIdentifier), so they keep
// working.
func Mode(db *store.DB) string {
	if db.GetSetting("identifier") == "samaccountname" {
		return "samaccountname"
	}
	return "mail"
}

// AvatarSafe replaces every character that is not a valid filename/URL path
// segment on Windows, macOS or Linux with an underscore, so an identifier can
// be used directly as "<id>.jpg" and in an avatarcache/ URL without encoding.
func AvatarSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`@<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
}

// MailIdentifier derives the identifier for a mail address: lowercased and
// made filename/URL safe.
func MailIdentifier(mail string) string {
	return AvatarSafe(strings.ToLower(strings.TrimSpace(mail)))
}

// UserIdentifier computes the active identifier for a user from their raw
// samaccountname and mail address, honouring the configured mode.
func UserIdentifier(db *store.DB, sam, mail string) string {
	if Mode(db) == "mail" {
		return MailIdentifier(mail)
	}
	return strings.TrimSpace(sam)
}

// MailResolver builds a lookup from the stored identifiers (the audit log
// User column, etc.) to the person's real e-mail address, so the UI can
// display "john.doe@teamviewer.com" instead of the stored value
// ("tvcorp\INT00234" in samaccountname mode, or "john.doe_teamviewer.com" in
// mail mode). It indexes the directory by both the active identifier and the
// raw samaccountname, and matches after stripping any DOMAIN\ prefix. The
// returned function returns its input unchanged when no directory match
// exists (e.g. "System", local admins).
func MailResolver(db *store.DB) func(string) string {
	dir, _ := db.ListDirectory()
	byKey := make(map[string]string, len(dir)*2)
	for _, d := range dir {
		mail := strings.TrimSpace(d.Mail)
		if mail == "" {
			continue
		}
		if d.Userid != "" {
			byKey[strings.ToLower(d.Userid)] = mail
		}
		if d.Samaccountname != "" {
			byKey[strings.ToLower(d.Samaccountname)] = mail
		}
	}
	return func(stored string) string {
		key := strings.TrimSpace(stored)
		if key == "" {
			return stored
		}
		if i := strings.LastIndex(key, "\\"); i >= 0 {
			key = key[i+1:]
		}
		if mail, ok := byKey[strings.ToLower(key)]; ok {
			return mail
		}
		return stored
	}
}
