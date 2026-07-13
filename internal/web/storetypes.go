package web

import "companymaps/internal/store"

// Transitional aliases: the data model lives in internal/store since the
// package restructure. These aliases keep the root-package handlers compiling
// unchanged until they move into internal/web (Phase 4), at which point this
// file is deleted and call sites import the store package directly.

type (
	DB              = store.DB
	MapInfo         = store.MapInfo
	Desk            = store.Desk
	LdapUser        = store.LdapUser
	DirectoryUser   = store.DirectoryUser
	Booking         = store.Booking
	Team            = store.Team
	Role            = store.Role
	User            = store.User
	ChangelogEntry  = store.ChangelogEntry
	StatEntry       = store.StatEntry
	VIP             = store.VIP
	RobinSpace      = store.RobinSpace
	RobinDeskStatus = store.RobinDeskStatus
	MeetingStatus   = store.MeetingStatus
	WhitelistEntry  = store.WhitelistEntry
	LdapSource      = store.LdapSource
	EntraSource     = store.EntraSource
	AuditEntry      = store.AuditEntry
	CustomItemType  = store.CustomItemType
	SourceRule      = store.SourceRule
)
