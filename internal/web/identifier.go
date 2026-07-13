package web

// Identifier-scheme wrappers. The logic lives in internal/directory; these
// App methods keep the existing handler call sites stable until they move to
// internal/web (Phase 4).

import (
	"strings"

	"companymaps/internal/directory"
	"companymaps/internal/store"
)

// identifierMode returns the configured identifier mode ("samaccountname" or
// "mail").
func (app *Server) identifierMode() string { return directory.Mode(app.db) }

// avatarSafe makes an identifier safe for use as a filename / URL segment.
func avatarSafe(s string) string { return directory.AvatarSafe(s) }

// mailIdentifier derives the identifier for a mail address.
func mailIdentifier(mail string) string { return directory.MailIdentifier(mail) }

// normalizeMail canonicalises an e-mail address for storage.
func normalizeMail(mail string) string { return store.NormalizeMail(mail) }

// normalizeMails canonicalises a slice of mail addresses.
func normalizeMails(mails []string) []string { return store.NormalizeMails(mails) }

// userIdentifier computes the active identifier for a user from their raw
// samaccountname and mail address, honouring the configured mode.
func (app *Server) userIdentifier(sam, mail string) string {
	return directory.UserIdentifier(app.db, sam, mail)
}

// sessionUserID returns the logged-in user's active bare identifier (the avatar
// filename base and booking key). It prefers the value stored on the session at
// login and falls back to stripping any DOMAIN\ prefix from the username.
func (app *Server) sessionUserID(sess Session) string {
	if sess.Samaccountname != "" {
		return sess.Samaccountname
	}
	u := sess.Username
	if i := strings.LastIndex(u, "\\"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

// mailResolver builds a lookup from stored identifiers to real mail addresses.
func (app *Server) mailResolver() func(string) string {
	return directory.MailResolver(app.db)
}
