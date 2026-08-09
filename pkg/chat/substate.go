package chat

import (
	"strings"

	"golift.io/subscribe"
)

// Meta keys motifini stores on a subscriber record.
const (
	metaKeyAuth        = "hasAuth"
	metaKeyDisplayName = "displayName"
	metaKeyUser        = "user"
)

// The helpers here read and write the subscribe.Subscriber fields that belong
// to motifini rather than the library: Meta, Contact, Admin and Ignored. Never
// touch those fields directly. Several goroutines share one record — Telegram
// dispatches every callback in its own, and each HTTP request runs in another —
// and the library also reads them while filtering subscribers and writing the
// state file. The subscribe accessors below hold the record's lock, which is
// the only thing both sides agree on.
//
// These wrappers exist to name motifini's concepts (an authed subscriber, a
// display name) and to keep the Meta keys in one place.

// SaveState writes the subscriber database to disk.
func SaveState(subs *subscribe.Subscribe) error {
	if subs == nil {
		return nil
	}

	return subs.StateFileSave() //nolint:wrapcheck // pass the library error through.
}

// SubAuthed reports whether a subscriber passed the /id password check or was
// allowed by an admin.
func SubAuthed(sub *subscribe.Subscriber) bool {
	value, _ := sub.GetMeta(metaKeyAuth)
	authed, _ := value.(bool)

	return authed
}

// SetSubAuthed records the outcome of an /id check or an admin Allow/Deny.
func SetSubAuthed(sub *subscribe.Subscriber, authed bool) {
	sub.SetMeta(metaKeyAuth, authed)
}

// SetSubDisplayName remembers the name the chat provider reports, so an admin
// listing can still name a subscriber whose Contact was never filled in.
func SetSubDisplayName(sub *subscribe.Subscriber, name string) {
	sub.SetMeta(metaKeyDisplayName, name)
}

// SetSubUser stores the chat provider's raw user record for later name recovery.
func SetSubUser(sub *subscribe.Subscriber, user any) {
	sub.SetMeta(metaKeyUser, user)
}

// SubAdmin reports whether a subscriber may run admin commands.
func SubAdmin(sub *subscribe.Subscriber) bool {
	return sub.IsAdmin()
}

// SetSubAdmin grants or revokes admin rights.
func SetSubAdmin(sub *subscribe.Subscriber, admin bool) {
	sub.SetAdmin(admin)
}

// SubIgnored reports whether a subscriber is excluded from the bot entirely.
func SubIgnored(sub *subscribe.Subscriber) bool {
	return sub.IsIgnored()
}

// SetSubIgnored ignores or unignores a subscriber.
func SetSubIgnored(sub *subscribe.Subscriber, ignored bool) {
	sub.SetIgnored(ignored)
}

// SubContact returns the subscriber's contact/display string.
func SubContact(sub *subscribe.Subscriber) string {
	return sub.GetContact()
}

// SetSubContact renames a subscriber.
func SetSubContact(sub *subscribe.Subscriber, name string) {
	sub.SetContact(name)
}

// EnsureSubContact fills a blank contact from the chat provider's display name.
// An existing contact (admin rename, earlier first/last name) is never clobbered.
func EnsureSubContact(sub *subscribe.Subscriber, name string) {
	sub.SetContactIfEmpty(strings.TrimSpace(name))
}

// SetSubMeta stores one application value on a subscriber record.
func SetSubMeta(sub *subscribe.Subscriber, key string, value any) {
	sub.SetMeta(key, value)
}

// DeleteSubMeta drops one application value from a subscriber record.
func DeleteSubMeta(sub *subscribe.Subscriber, key string) {
	sub.DeleteMeta(key)
}
