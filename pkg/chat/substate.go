package chat

import (
	"strings"
	"sync"

	"golift.io/subscribe"
)

// Meta keys motifini stores on a subscriber record.
const (
	metaKeyAuth        = "hasAuth"
	metaKeyDisplayName = "displayName"
	metaKeyUser        = "user"
)

// subMu guards the subscribe.Subscriber fields that belong to the application
// rather than the library: Meta, Contact, Admin and Ignored. The subscribe
// library never locks them (it only copies them while writing the state file),
// and several goroutines touch the same records: Telegram dispatches every
// callback in its own goroutine, and each HTTP request runs in another. Read
// and write those fields through the helpers here so the state file save and
// the readers can never observe a half-written Meta map.
//
//nolint:gochecknoglobals // one lock for records shared by every goroutine.
var subMu sync.RWMutex

// SaveState writes the subscriber database to disk. It holds the read lock
// because the save copies Meta, Contact, Admin and Ignored out of every record.
func SaveState(subs *subscribe.Subscribe) error {
	if subs == nil {
		return nil
	}

	subMu.RLock()
	defer subMu.RUnlock()

	return subs.StateFileSave() //nolint:wrapcheck // pass the library error through.
}

// SubAuthed reports whether a subscriber passed the /id password check or was
// allowed by an admin.
func SubAuthed(sub *subscribe.Subscriber) bool {
	if sub == nil {
		return false
	}

	subMu.RLock()
	defer subMu.RUnlock()

	authed, _ := sub.Meta[metaKeyAuth].(bool)

	return authed
}

// SetSubAuthed records the outcome of an /id check or an admin Allow/Deny.
func SetSubAuthed(sub *subscribe.Subscriber, authed bool) {
	SetSubMeta(sub, metaKeyAuth, authed)
}

// SetSubDisplayName remembers the name the chat provider reports, so an admin
// listing can still name a subscriber whose Contact was never filled in.
func SetSubDisplayName(sub *subscribe.Subscriber, name string) {
	SetSubMeta(sub, metaKeyDisplayName, name)
}

// SetSubUser stores the chat provider's raw user record for later name recovery.
func SetSubUser(sub *subscribe.Subscriber, user any) {
	SetSubMeta(sub, metaKeyUser, user)
}

// SubAdmin reports whether a subscriber may run admin commands.
func SubAdmin(sub *subscribe.Subscriber) bool {
	if sub == nil {
		return false
	}

	subMu.RLock()
	defer subMu.RUnlock()

	return sub.Admin
}

// SetSubAdmin grants or revokes admin rights.
func SetSubAdmin(sub *subscribe.Subscriber, admin bool) {
	if sub == nil {
		return
	}

	subMu.Lock()
	defer subMu.Unlock()

	sub.Admin = admin
}

// SubIgnored reports whether a subscriber is excluded from the bot entirely.
func SubIgnored(sub *subscribe.Subscriber) bool {
	if sub == nil {
		return false
	}

	subMu.RLock()
	defer subMu.RUnlock()

	return sub.Ignored
}

// SetSubIgnored ignores or unignores a subscriber.
func SetSubIgnored(sub *subscribe.Subscriber, ignored bool) {
	if sub == nil {
		return
	}

	subMu.Lock()
	defer subMu.Unlock()

	sub.Ignored = ignored
}

// SubContact returns the subscriber's contact/display string.
func SubContact(sub *subscribe.Subscriber) string {
	if sub == nil {
		return ""
	}

	subMu.RLock()
	defer subMu.RUnlock()

	return sub.Contact
}

// SetSubContact renames a subscriber.
func SetSubContact(sub *subscribe.Subscriber, name string) {
	if sub == nil {
		return
	}

	subMu.Lock()
	defer subMu.Unlock()

	sub.Contact = name
}

// EnsureSubContact fills a blank contact from the chat provider's display name.
// An existing contact (admin rename, earlier first/last name) is never clobbered.
func EnsureSubContact(sub *subscribe.Subscriber, name string) {
	name = strings.TrimSpace(name)
	if sub == nil || name == "" {
		return
	}

	subMu.Lock()
	defer subMu.Unlock()

	if strings.TrimSpace(sub.Contact) == "" {
		sub.Contact = name
	}
}

// SetSubMeta stores one application value on a subscriber record.
func SetSubMeta(sub *subscribe.Subscriber, key string, value any) {
	if sub == nil {
		return
	}

	subMu.Lock()
	defer subMu.Unlock()

	subMeta(sub)[key] = value
}

// DeleteSubMeta drops one application value from a subscriber record.
func DeleteSubMeta(sub *subscribe.Subscriber, key string) {
	if sub == nil {
		return
	}

	subMu.Lock()
	defer subMu.Unlock()

	delete(sub.Meta, key)
}

// subMeta returns the Meta map, creating it when absent.
// Callers must hold subMu for writing.
func subMeta(sub *subscribe.Subscriber) map[string]any {
	if sub.Meta == nil {
		sub.Meta = map[string]any{}
	}

	return sub.Meta
}
