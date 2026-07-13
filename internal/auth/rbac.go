package auth

import "companymaps/internal/store"

// PermLevel returns the permission level (0=none, 1=read, 2=write) the
// session's user holds for a feature. The break-glass config.json admin
// always gets 2.
func PermLevel(db *store.DB, sess Session, feature string) int {
	if sess.AdminPassword {
		return 2
	}
	if sess.Username == "" {
		return 0
	}
	u, ok, err := db.GetUser(sess.Username)
	if err != nil || !ok {
		return 0
	}
	role, ok, err := db.GetRole(u.Role)
	if err != nil || !ok {
		return 0
	}
	return role.Perms[feature]
}
