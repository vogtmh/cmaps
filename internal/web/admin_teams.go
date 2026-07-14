package web

import (
	"companymaps/internal/store"
	"net/http"
	"strings"
)

// handleAdminPostTeams handles team create/update/delete.
func (app *Server) handleAdminPostTeams(r *http.Request, sess Session) string {
	if app.permLevel(sess, "teams") < 2 {
		return ""
	}
	if del := r.FormValue("deleteTeam"); del != "" {
		_ = app.db.DeleteTeam(del)
		_ = app.db.AuditLog("Teams", sess.Username, "Team removed ("+del+")")
		return "Team removed."
	}
	if orig := r.FormValue("editTeamOrigName"); orig != "" {
		name := strings.TrimSpace(r.FormValue("editTeamName"))
		if name == "" {
			return ""
		}
		members := normalizeMembers(r.FormValue("editTeamMembers"))
		if name != orig {
			_ = app.db.DeleteTeam(orig)
		}
		_ = app.db.PutTeam(store.Team{Teamname: name, Members: members})
		_ = app.db.AuditLog("Teams", sess.Username, "Team updated ("+name+")")
		return "Team updated."
	}
	name := strings.TrimSpace(r.FormValue("newTeam"))
	if name != "" {
		members := normalizeMembers(r.FormValue("newMembers"))
		_ = app.db.PutTeam(store.Team{Teamname: name, Members: members})
		_ = app.db.AuditLog("Teams", sess.Username, "New team created ("+name+")")
		return "Team created."
	}
	return ""
}

func normalizeMembers(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "|")
}

// saveLogosFromForm stores any uploaded logo images and points the matching
