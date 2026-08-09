package chat

import (
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"golift.io/subscribe"
)

func TestSubStateAccessors(t *testing.T) {
	t.Parallel()

	sub := &subscribe.Subscriber{ID: 42, API: "telegram"}

	if SubAuthed(sub) || SubAdmin(sub) || SubIgnored(sub) {
		t.Fatal("a fresh subscriber should have no flags set")
	}

	SetSubAuthed(sub, true)
	SetSubAdmin(sub, true)
	SetSubIgnored(sub, true)

	if !SubAuthed(sub) || !SubAdmin(sub) || !SubIgnored(sub) {
		t.Fatalf("flags did not stick: authed=%v admin=%v ignored=%v",
			SubAuthed(sub), SubAdmin(sub), SubIgnored(sub))
	}

	EnsureSubContact(sub, "first")
	EnsureSubContact(sub, "second")

	if got := SubContact(sub); got != "first" {
		t.Fatalf("EnsureSubContact overwrote an existing contact: %q", got)
	}

	SetSubContact(sub, "renamed")

	if got := SubContact(sub); got != "renamed" {
		t.Fatalf("SetSubContact: got %q want renamed", got)
	}

	SetSubMeta(sub, pendingRenameMetaKey, int64(7))

	if id, ok := pendingRenameID(sub); !ok || id != 7 {
		t.Fatalf("pendingRenameID: got %d,%v want 7,true", id, ok)
	}

	DeleteSubMeta(sub, pendingRenameMetaKey)

	if _, ok := pendingRenameID(sub); ok {
		t.Fatal("pending rename should be gone after DeleteSubMeta")
	}
}

// Nil subscribers reach these helpers from callback paths where the record was
// deleted mid-conversation.
func TestSubStateNilSafe(t *testing.T) {
	t.Parallel()

	if SubAuthed(nil) || SubAdmin(nil) || SubIgnored(nil) || SubContact(nil) != "" {
		t.Fatal("nil subscriber should read as empty")
	}

	SetSubAuthed(nil, true)
	SetSubAdmin(nil, true)
	SetSubIgnored(nil, true)
	SetSubContact(nil, "x")
	EnsureSubContact(nil, "x")
	SetSubMeta(nil, "k", "v")
	SetSubDisplayName(nil, "x")
	SetSubUser(nil, "x")
	DeleteSubMeta(nil, "k")

	err := SaveState(nil)
	if err != nil {
		t.Fatalf("SaveState(nil): %v", err)
	}
}

// subStateRounds is how many times each concurrent worker loops.
const subStateRounds = 40

// TestSubStateConcurrentAccess is a race-detector test: Telegram runs every
// callback in its own goroutine while HTTP requests read the same records and
// save the state file, which copies Meta out of each one.
func TestSubStateConcurrentAccess(t *testing.T) {
	t.Parallel()

	subs, err := subscribe.GetDB(filepath.Join(t.TempDir(), "subscribers.json"))
	if err != nil {
		t.Fatalf("subscribe.GetDB: %v", err)
	}

	var wait sync.WaitGroup

	for num := range 4 {
		person := subs.CreateSubWithID(int64(num+1), "sub"+strconv.Itoa(num), "telegram", false, false)

		wait.Go(func() { writeSubState(person) })
		wait.Go(func() { readSubState(person) })
		wait.Go(func() { saveSubState(t, subs) })
	}

	wait.Wait()
}

// writeSubState is an admin flipping flags from a Telegram callback.
func writeSubState(person *subscribe.Subscriber) {
	for round := range subStateRounds {
		SetSubAuthed(person, round%2 == 0)
		SetSubAdmin(person, round%3 == 0)
		SetSubIgnored(person, round%5 == 0)
		SetSubDisplayName(person, "name"+strconv.Itoa(round))
		SetSubUser(person, map[string]any{"username": "u" + strconv.Itoa(round)})
		SetSubMeta(person, pendingRenameMetaKey, int64(round))
		DeleteSubMeta(person, pendingRenameMetaKey)
	}
}

// readSubState is the HTTP API deciding whether an id may receive messages,
// plus the admin listings that render the same record.
func readSubState(person *subscribe.Subscriber) {
	for range subStateRounds {
		_ = SubAuthed(person) && !SubIgnored(person)
		_ = SubAdmin(person)
		_ = SubContact(person)
		_ = subscriberDisplayName(person)
		_ = adminSubSummary(person)
	}
}

// saveSubState is any handler that persists the database after a change.
func saveSubState(t *testing.T, subs *subscribe.Subscribe) {
	t.Helper()

	for range subStateRounds {
		err := SaveState(subs)
		if err != nil {
			t.Errorf("SaveState: %v", err)

			return
		}
	}
}
