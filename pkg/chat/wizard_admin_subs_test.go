package chat

import (
	"fmt"
	"testing"
	"time"

	"golift.io/subscribe"
)

func TestHandleAdminSubsClearsPendingRename(t *testing.T) {
	t.Parallel()

	admin, target, chat := adminSubsTestFixture(t)
	SetSubMeta(admin, pendingRenameMetaKey, target.ID)

	handler := &Handler{API: "telegram", Sub: admin}
	reply, save, ok := chat.handleAdminSubsWizardCallback(handler, fmt.Sprintf("m:subs:%d", target.ID))
	if !ok || reply == nil {
		t.Fatalf("expected handled callback, ok=%v reply=%v", ok, reply)
	}
	if save {
		t.Fatal("listing subs should not save")
	}
	if _, pending := pendingRenameID(admin); pending {
		t.Fatal("pending rename should be cleared when entering manage-subs")
	}
}

func TestAdminSubsWizardPauseValidatesMinutes(t *testing.T) {
	t.Parallel()

	admin, target, chat := adminSubsTestFixture(t)
	handler := &Handler{API: "telegram", Sub: admin}
	uid := target.ID

	// Names() is sorted; Office:human is the only event → index 0.
	reply, save := chat.adminSubsWizardPause(handler, fmt.Sprintf("%d:nope:0", uid))
	if save || reply == nil || reply.Toast != "Error" {
		t.Fatalf("bad mins: save=%v toast=%q reply=%q", save, reply.Toast, reply.Reply)
	}

	reply, save = chat.adminSubsWizardPause(handler, fmt.Sprintf("%d:-1:0", uid))
	if save || reply.Toast != "Error" {
		t.Fatalf("negative mins: save=%v toast=%q", save, reply.Toast)
	}

	reply, save = chat.adminSubsWizardPause(handler, fmt.Sprintf("%d:%d:0", uid, MaxPauseMinutes+1))
	if save || reply.Toast != "Error" {
		t.Fatalf("too large mins: save=%v toast=%q", save, reply.Toast)
	}

	if target.Events.IsPaused("Office:human") {
		t.Fatal("invalid pause must not change subscription")
	}
}

func TestAdminSubsWizardPauseAppliesAndHandlesMissing(t *testing.T) {
	t.Parallel()

	admin, target, chat := adminSubsTestFixture(t)
	handler := &Handler{API: "telegram", Sub: admin}
	uid := target.ID

	reply, save := chat.adminSubsWizardPause(handler, fmt.Sprintf("%d:10:0", uid))
	if !save || reply.Toast != "Saved" {
		t.Fatalf("valid pause: save=%v toast=%q", save, reply.Toast)
	}
	if !target.Events.IsPaused("Office:human") {
		t.Fatal("expected Office:human paused")
	}

	reply, save = chat.adminSubsWizardPause(handler, fmt.Sprintf("%d:0:0", uid))
	if !save || reply.Toast != "Saved" {
		t.Fatalf("clear pause: save=%v toast=%q", save, reply.Toast)
	}
	// Pause(0) sets Pause=now; IsPaused is true only when Pause.After(now), so
	// a just-cleared pause should not still count as paused.
	if target.Events.IsPaused("Office:human") {
		// Allow tiny clock skew: wait and re-check once.
		time.Sleep(2 * time.Millisecond)
		if target.Events.IsPaused("Office:human") {
			t.Fatal("expected pause cleared")
		}
	}

	// Stale index after the subscription is removed.
	target.Events.Remove("Office:human")
	reply, save = chat.adminSubsWizardPause(handler, fmt.Sprintf("%d:10:0", uid))
	if save || reply.Toast != "Missing" {
		t.Fatalf("missing sub: save=%v toast=%q", save, reply.Toast)
	}
}

func adminSubsTestFixture(t *testing.T) (*subscribe.Subscriber, *subscribe.Subscriber, *Chat) {
	t.Helper()

	admin := &subscribe.Subscriber{
		ID:      1,
		API:     "telegram",
		Contact: "Admin",
		Admin:   true,
		Meta:    map[string]any{},
		Events:  &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	target := &subscribe.Subscriber{
		ID:      2,
		API:     "telegram",
		Contact: "Alice",
		Events:  &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	err := target.Subscribe("Office:human")
	if err != nil {
		t.Fatal(err)
	}

	chat := &Chat{
		Subs: &subscribe.Subscribe{
			Subscribers: []*subscribe.Subscriber{admin, target},
			Events:      &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
		},
	}

	return admin, target, chat
}
