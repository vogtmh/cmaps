package main

import "strings"

// The employee "common identifier" is either the samaccountname (default, only
// available via LDAP) or the e-mail address (available via LDAP, EntraID and SAML
// SSO). It is the key used for avatar filenames, the audit log, map-admin
// records, bookings and the changelog.
//
// So the identifier is always a valid filename and URL path segment on Windows,
// macOS and Linux, the mail-mode identifier is the lowercased mail address with
// every filesystem/URL-unsafe character (notably "@") replaced by "_", e.g.
// "john.doe@teamviewer.com" -> "john.doe_teamviewer.com". The raw mail address is
// always kept separately (the Mail field) for display and contact. samaccountname
// identifiers are used verbatim (they never contain unsafe characters).

// identifierMode returns the configured identifier mode, defaulting to
// "samaccountname" (so existing installs, which have no setting, keep working).
func (app *App) identifierMode() string {
	if app.db.GetSetting("identifier") == "mail" {
		return "mail"
	}
	return "samaccountname"
}

// avatarSafe replaces every character that is not a valid filename/URL path
// segment on Windows, macOS or Linux with an underscore, so an identifier can be
// used directly as "<id>.jpg" and in an avatarcache/ URL without encoding.
func avatarSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`@<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
}

// mailIdentifier derives the identifier for a mail address: lowercased and made
// filename/URL safe.
func mailIdentifier(mail string) string {
	return avatarSafe(strings.ToLower(strings.TrimSpace(mail)))
}

// normalizeMail canonicalises an e-mail address for storage: trimmed and strictly
// lowercased. Mail addresses are case-insensitive for our purposes (matching,
// identifiers, display), so every directory source (LDAP, EntraID, SAML, Robin)
// runs its mail values through this before persisting, and it is applied again
// defensively at the storage layer in case a value slips through another path.
func normalizeMail(mail string) string {
	return strings.ToLower(strings.TrimSpace(mail))
}

// normalizeMails canonicalises a slice of mail addresses (e.g. AD proxyAddresses
// aliases), dropping any that are empty after trimming.
func normalizeMails(mails []string) []string {
	out := make([]string, 0, len(mails))
	for _, m := range mails {
		if n := normalizeMail(m); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// userIdentifier computes the active identifier for a user from their raw
// samaccountname and mail address, honouring the configured mode.
func (app *App) userIdentifier(sam, mail string) string {
	if app.identifierMode() == "mail" {
		return mailIdentifier(mail)
	}
	return strings.TrimSpace(sam)
}

// sessionUserID returns the logged-in user's active bare identifier (the avatar
// filename base and booking key). It prefers the value stored on the session at
// login and falls back to stripping any DOMAIN\ prefix from the username.
func (app *App) sessionUserID(sess Session) string {
	if sess.Samaccountname != "" {
		return sess.Samaccountname
	}
	u := sess.Username
	if i := strings.LastIndex(u, "\\"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

// mailResolver builds a lookup from the stored identifiers (the audit log User
// column, etc.) to the person's real e-mail address, so the UI can display
// "john.doe@teamviewer.com" instead of the stored value ("tvcorp\INT00234" in
// samaccountname mode, or "john.doe_teamviewer.com" in mail mode). It indexes the
// directory by both the active identifier and the raw samaccountname, and matches
// after stripping any DOMAIN\ prefix. The returned function returns its input
// unchanged when no directory match exists (e.g. "System", local admins).
func (app *App) mailResolver() func(string) string {
	dir, _ := app.db.ListDirectory()
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
