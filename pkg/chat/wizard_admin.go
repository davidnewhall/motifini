package chat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golift.io/subscribe"
)

// Admin subscriber-management wizard callbacks (Telegram ≤64 bytes).
const (
	cbUsersRoot = "m"
)

func (c *Chat) usersWizardRoot(handler *Handler) *Reply {
	if handler == nil || handler.Sub == nil || !handler.Sub.Admin {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}
	}

	subs := c.Subs.Subscribers
	rows := make([][]Button, 0, len(subs)+1)

	var msg strings.Builder
	fmt.Fprintf(&msg, "Subscriber management (%d).\n\n", len(subs))
	msg.WriteString("Tap a person for details and actions.\n")
	msg.WriteString("★ admin · ⊘ ignored · ? not authenticated")

	for _, sub := range subs {
		if sub == nil {
			continue
		}

		rows = append(rows, []Button{{
			Label: adminSubButtonLabel(sub),
			Data:  fmt.Sprintf("m:i:%d", sub.ID),
		}})
	}

	if len(rows) == 0 {
		msg.WriteString("\n\n(none yet)")
	}

	rows = append(rows, []Button{{Label: "Done", Data: cbCancel}})

	return &Reply{Reply: msg.String(), Edit: true, Keyboard: rows}
}

func (c *Chat) usersWizardItem(handler *Handler, idStr string) *Reply {
	if handler == nil || handler.Sub == nil || !handler.Sub.Admin {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return &Reply{
			Reply: "Subscriber gone — try again.", Edit: true, Toast: "Missing",
			Keyboard: [][]Button{{{Label: "« Back", Data: cbUsersRoot}}},
		}
	}

	auth := subscriberHasAuth(target)
	self := handler.Sub.ID == target.ID && handler.Sub.API == target.API

	var msg strings.Builder
	fmt.Fprintf(&msg, "%s\n\n", adminSubDetail(target))
	msg.WriteString("Choose an action:")

	rows := [][]Button{}
	if !auth {
		rows = append(rows, []Button{{Label: "Allow", Data: fmt.Sprintf("m:allow:%d", target.ID)}})
	} else {
		rows = append(rows, []Button{{Label: "Deny (revoke /id)", Data: fmt.Sprintf("m:deny:%d", target.ID)}})
	}

	if target.Ignored {
		rows = append(rows, []Button{{Label: "Unignore", Data: fmt.Sprintf("m:unignore:%d", target.ID)}})
	} else {
		rows = append(rows, []Button{{Label: "Ignore", Data: fmt.Sprintf("m:ignore:%d", target.ID)}})
	}

	if target.Admin {
		rows = append(rows, []Button{{Label: "Remove admin", Data: fmt.Sprintf("m:unadmin:%d", target.ID)}})
	} else {
		rows = append(rows, []Button{{Label: "Make admin", Data: fmt.Sprintf("m:admin:%d", target.ID)}})
	}

	rows = append(rows,
		[]Button{{Label: "Rename…", Data: fmt.Sprintf("m:rename:%d", target.ID)}},
		[]Button{{Label: "Delete…", Data: fmt.Sprintf("m:del:%d", target.ID)}},
		[]Button{
			{Label: "« Back", Data: cbUsersRoot},
			{Label: "Done", Data: cbCancel},
		},
	)

	if self {
		msg.WriteString("\n\n(This is you — some actions are blocked.)")
	}

	return &Reply{Reply: msg.String(), Edit: true, Keyboard: rows}
}

func (c *Chat) usersWizardConfirmDelete(handler *Handler, idStr string) *Reply {
	if handler == nil || handler.Sub == nil || !handler.Sub.Admin {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return &Reply{
			Reply: "Subscriber gone.", Edit: true, Toast: "Missing",
			Keyboard: [][]Button{{{Label: "« Back", Data: cbUsersRoot}}},
		}
	}

	return &Reply{
		Reply: fmt.Sprintf(
			"Delete %s (id %d) permanently?\n\n"+
				"This removes their record and all subscriptions. "+
				"They would need to message the bot again to reappear.",
			subscriberDisplayName(target), target.ID),
		Edit:  true,
		Toast: "Confirm",
		Keyboard: [][]Button{
			{{Label: "Yes, delete", Data: fmt.Sprintf("m:delok:%d", target.ID)}},
			{{Label: "Cancel", Data: fmt.Sprintf("m:i:%d", target.ID)}},
		},
	}
}

func (c *Chat) usersWizardAction(handler *Handler, action, idStr string) (*Reply, bool) {
	if handler == nil || handler.Sub == nil || !handler.Sub.Admin {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}, false
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return &Reply{
			Reply: "Subscriber gone.", Edit: true, Toast: "Missing",
			Keyboard: [][]Button{{{Label: "« Back", Data: cbUsersRoot}}},
		}, false
	}

	if action == "delok" {
		return c.usersWizardDelete(handler, target)
	}

	msg, toast, blocked := c.applyUsersAction(handler, target, action)
	if blocked != "" {
		return c.usersWizardBlocked(handler, target, blocked), false
	}
	if msg == "" {
		return &Reply{Reply: "Unknown action.", Edit: true, Toast: "??"}, false
	}

	next := c.usersWizardItem(handler, idStr)
	next.Reply = msg + "\n\n" + next.Reply
	next.Toast = toast

	return next, true
}

func (c *Chat) usersWizardDelete(handler *Handler, target *subscribe.Subscriber) (*Reply, bool) {
	name := subscriberDisplayName(target)
	if handler.Sub.ID == target.ID && handler.Sub.API == target.API {
		return c.usersWizardBlocked(handler, target, "You can't delete yourself."), false
	}
	if target.Admin && len(c.Subs.GetAdmins()) <= 1 {
		return c.usersWizardBlocked(handler, target, "Can't delete the last admin."), false
	}
	err := c.Subs.DeleteSubscriber(target.ID, target.API)
	if err != nil {
		return &Reply{
			Reply: "Delete failed: " + err.Error(), Edit: true, Toast: "Error",
			Keyboard: [][]Button{{{Label: "« Back", Data: cbUsersRoot}}},
		}, false
	}

	next := c.usersWizardRoot(handler)
	next.Reply = fmt.Sprintf("Deleted %s.\n\n", name) + next.Reply
	next.Toast = "Deleted"

	return next, true
}

func (c *Chat) applyUsersAction(
	handler *Handler, target *subscribe.Subscriber, action string,
) (string, string, string) {
	self := handler.Sub.ID == target.ID && handler.Sub.API == target.API
	name := subscriberDisplayName(target)

	switch action {
	case "allow":
		ensureSubMeta(target)["hasAuth"] = true
		target.Ignored = false

		return fmt.Sprintf("Allowed %s — they can use the bot now.", name), "Allowed", ""

	case "deny":
		if self {
			return "", "", "You can't deny yourself."
		}
		ensureSubMeta(target)["hasAuth"] = false

		return fmt.Sprintf("Denied %s — they need /id or another Allow.", name), "Denied", ""

	case "ignore":
		if self {
			return "", "", "You can't ignore yourself."
		}
		target.Ignored = true
		target.Admin = false

		return fmt.Sprintf("Ignored %s (also removed admin).", name), "Ignored", ""

	case "unignore":
		target.Ignored = false

		return fmt.Sprintf("Unignored %s.", name), "Unignored", ""

	case "admin":
		target.Admin = true
		target.Ignored = false
		ensureSubMeta(target)["hasAuth"] = true

		return name + " is now an admin.", "Admin", ""

	case "unadmin":
		if self {
			return "", "", "You can't remove your own admin."
		}
		if len(c.Subs.GetAdmins()) <= 1 && target.Admin {
			return "", "", "Can't remove the last admin."
		}
		target.Admin = false

		return name + " is no longer an admin.", "Unadmin", ""

	default:
		return "", "", ""
	}
}

func ensureSubMeta(sub *subscribe.Subscriber) map[string]any {
	if sub.Meta == nil {
		sub.Meta = map[string]any{}
	}

	return sub.Meta
}

func (c *Chat) usersWizardRenamePrompt(handler *Handler, idStr string) *Reply {
	if handler == nil || handler.Sub == nil || !handler.Sub.Admin {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return &Reply{
			Reply: "Subscriber gone.", Edit: true, Toast: "Missing",
			Keyboard: [][]Button{{{Label: "« Back", Data: cbUsersRoot}}},
		}
	}

	ensureSubMeta(handler.Sub)[pendingRenameMetaKey] = target.ID

	return &Reply{
		Reply: fmt.Sprintf(
			"Rename %s (id %d).\n\nSend the new display name as your next message.\n"+
				"Example: Torres\n\nOr tap Cancel.",
			subscriberDisplayName(target), target.ID),
		Edit:  true,
		Toast: "Type a name",
		Keyboard: [][]Button{
			{{Label: "Cancel", Data: fmt.Sprintf("m:i:%d", target.ID)}},
		},
	}
}

const pendingRenameMetaKey = "pendingRenameID"

func clearPendingRename(sub *subscribe.Subscriber) {
	if sub == nil || sub.Meta == nil {
		return
	}

	delete(sub.Meta, pendingRenameMetaKey)
}

func pendingRenameID(sub *subscribe.Subscriber) (int64, bool) {
	if sub == nil || sub.Meta == nil {
		return 0, false
	}

	return metaInt64(sub.Meta, pendingRenameMetaKey)
}

func metaInt64(meta map[string]any, key string) (int64, bool) {
	val, exists := meta[key]
	if !exists || val == nil {
		return 0, false
	}

	intVal, exists := anyToInt64(val)

	return intVal, exists && intVal != 0
}

//nolint:varnamelen // v is the value to convert to int64.
func anyToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()

		return i, err == nil
	case string:
		i, err := strconv.ParseInt(n, 10, 64)

		return i, err == nil
	default:
		return 0, false
	}
}

// consumePendingRename applies an admin's next free-text message as a rename target.
// handled is false when no pending rename is active.
func (c *Chat) consumePendingRename(handler *Handler) (*Reply, bool, bool) {
	if handler == nil || handler.Sub == nil || !handler.Sub.Admin || len(handler.Text) == 0 {
		return nil, false, false
	}

	renameID, ok := pendingRenameID(handler.Sub)
	if !ok {
		return nil, false, false
	}

	// Slash commands cancel the rename prompt and fall through.
	if strings.HasPrefix(handler.Text[0], "/") {
		clearPendingRename(handler.Sub)

		return nil, true, true
	}

	name := strings.TrimSpace(strings.Join(handler.Text, " "))
	clearPendingRename(handler.Sub)

	if name == "" {
		return &Reply{
			Reply:    "Name was empty — rename cancelled.",
			Keyboard: [][]Button{{{Label: "Users", Data: cbUsersRoot}}},
		}, true, true
	}

	target, err := c.Subs.GetSubscriberByID(renameID, handler.API)
	if err != nil {
		return &Reply{
			Reply:    "That subscriber is gone — rename cancelled.",
			Keyboard: [][]Button{{{Label: "Users", Data: cbUsersRoot}}},
		}, true, true
	}

	old := subscriberDisplayName(target)
	target.Contact = name
	ensureSubMeta(target)["displayName"] = name

	next := c.usersWizardItem(handler, strconv.FormatInt(renameID, 10))
	next.Edit = false // new message after free-text
	next.Reply = fmt.Sprintf("Renamed %s → %s\n\n", old, name) + next.Reply
	next.Toast = "Renamed"

	return next, true, true
}

func (c *Chat) usersWizardBlocked(handler *Handler, target *subscribe.Subscriber, why string) *Reply {
	next := c.usersWizardItem(handler, strconv.FormatInt(target.ID, 10))
	next.Reply = why + "\n\n" + next.Reply
	next.Toast = "Blocked"

	return next
}

func (c *Chat) adminTargetByID(api, idStr string) (*subscribe.Subscriber, error) {
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetID == 0 {
		return nil, subscribe.ErrSubscriberNotFound
	}

	sub, err := c.Subs.GetSubscriberByID(targetID, api)
	if err != nil {
		return nil, fmt.Errorf("get subscriber %d: %w", targetID, err)
	}

	return sub, nil
}

func subscriberDisplayName(sub *subscribe.Subscriber) string {
	if sub == nil {
		return "?"
	}

	if name := strings.TrimSpace(sub.Contact); name != "" {
		return name
	}

	// Recover a name from Meta and persist it onto Contact.
	if name := metaDisplayName(sub.Meta); name != "" {
		sub.Contact = name

		return name
	}

	return strconv.FormatInt(sub.ID, 10)
}

// metaDisplayName pulls a human name from Meta (displayName string or Telegram user dump).
func metaDisplayName(meta map[string]any) string {
	if meta == nil {
		return ""
	}

	if s, _ := meta["displayName"].(string); strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}

	return telegramUserMetaName(meta)
}

// telegramUserMetaName pulls username / first+last from Meta["user"] (Telegram User dump).
func telegramUserMetaName(meta map[string]any) string {
	if meta == nil {
		return ""
	}

	raw, exists := meta["user"]
	if !exists || raw == nil {
		return ""
	}

	user, exists := raw.(map[string]any)
	if !exists {
		return ""
	}

	if u := metaString(user, "username", "UserName"); u != "" {
		return u
	}

	first := metaString(user, "first_name", "FirstName")
	last := metaString(user, "last_name", "LastName")

	return strings.TrimSpace(first + " " + last)
}

func metaString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, _ := m[k].(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}

	return ""
}

func subscriberHasAuth(sub *subscribe.Subscriber) bool {
	if sub == nil || sub.Meta == nil {
		return false
	}
	a, _ := sub.Meta["hasAuth"].(bool)

	return a
}

func formatFirstSeen(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	//nolint:gosmopolitan // local time is fine for admin display.
	return t.Local().Format("2006-01-02 15:04")
}

func adminSubFlags(sub *subscribe.Subscriber) string {
	var parts []string
	if subscriberHasAuth(sub) {
		parts = append(parts, "auth")
	} else {
		parts = append(parts, "no-auth")
	}
	if sub.Admin {
		parts = append(parts, "admin")
	}
	if sub.Ignored {
		parts = append(parts, "ignored")
	}

	return strings.Join(parts, ", ")
}

func adminSubSummary(sub *subscribe.Subscriber) string {
	nEvents := 0
	if sub.Events != nil {
		nEvents = sub.Events.Len()
	}

	return fmt.Sprintf("%s · id %d · %s · first %s · %d subs",
		subscriberDisplayName(sub), sub.ID, adminSubFlags(sub), formatFirstSeen(sub.FirstSeen), nEvents)
}

func adminSubDetail(sub *subscribe.Subscriber) string {
	nEvents := 0
	if sub.Events != nil {
		nEvents = sub.Events.Len()
	}

	return fmt.Sprintf(
		"%s\nID: %d\nAPI: %s\nFlags: %s\nFirst seen: %s\nSubscriptions: %d",
		subscriberDisplayName(sub), sub.ID, sub.API, adminSubFlags(sub),
		formatFirstSeen(sub.FirstSeen), nEvents)
}

func adminSubButtonLabel(sub *subscribe.Subscriber) string {
	label := subscriberDisplayName(sub)
	if sub.Admin {
		label += " ★"
	}
	if sub.Ignored {
		label += " ⊘"
	}
	if !subscriberHasAuth(sub) {
		label += " ?"
	}
	// Telegram button labels max out around 64 runes; keep room for badges.
	runes := []rune(label)
	if len(runes) > 60 {
		label = string(runes[:57]) + "…"
	}

	return label
}
