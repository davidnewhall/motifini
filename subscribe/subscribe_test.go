package subscribe

/* TODO: a few new methods require tests. */
import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testFile = "/tmp/this_is_a_testfile_for_subtscribe_test.go.json"
var testFile2 = "/tmp/this_is_a_testfile_for_subtscribe_test2.go.json"

func TestCheckAPI(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	a.True(checkAPI("test_string", []string{}), "an empty slice must always return true")
	a.True(checkAPI("test_string://event", []string{"event", "test_string"}), "test_string is an allowed api prefix")
	a.True(checkAPI("test_string", []string{"event", "any"}), "any as a slice value must return true")
	a.True(checkAPI("test_string", []string{"event", "all"}), "all as a slice value must return true")
	a.True(checkAPI("test_string", []string{"event", "test_string"}), "test_string is an allowed api")
	a.False(checkAPI("test_string", []string{"event", "test_string2"}), "test_string is not an allowed api")
}

func TestGetDB(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub, err := GetDB([]string{}, "")
	a.Nil(err, "getting an empty db must produce no error")
	json, err := sub.GetStateJSON()
	a.EqualValues(`{"events":{},"subscribers":[]}`, string(json), "the initial state must be empty")
	a.Nil(err, "getting an empty state must produce no error")
}

func TestError(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sentence := "this is an error string"
	err := Error(sentence)
	a.EqualValues(sentence, err.Error())
}

func TestLoadStateFile(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	// test with good data.
	testJSON := `{"events":{},"subscribers":[{"api":"http","contact":"testUser","events":{},"is_admin":false,"ignored":false}]}`
	a.Nil(ioutil.WriteFile(testFile, []byte(testJSON), 0644), "problem writing test file")
	sub, err := GetDB([]string{}, testFile)
	a.Nil(err, "there must be no error loading the state file")
	json, err := sub.GetStateJSON()
	a.Nil(err, "there must be no error getting the state data")
	a.EqualValues(testJSON, string(json))
	// Test missing file.
	a.Nil(os.RemoveAll(testFile), "problem removing test file")
	sub, err = GetDB([]string{}, testFile)
	a.Nil(err, "there must be no error when the state file is missing")
	data, err := ioutil.ReadFile(testFile)
	a.Nil(err, "error reading test file")
	a.EqualValues(`{"events":{},"subscribers":[]}`, string(data), "the initial state file must be empty")
	// Test uncreatable file.
	_, err = GetDB([]string{}, "/tmp/xxx/yyy/zzz/aaa/bbb/this_file_dont_exist")
	a.NotNil(err, "there must be an error when the state cannot be created")
	// Test unreadable file.
	_, err = GetDB([]string{}, "/etc/sudoers")
	a.NotNil(err, "there must be an error when the state cannot be read")
	// Test bad data.
	err = ioutil.WriteFile(testFile, []byte("this aint good json}}"), 0644)
	a.Nil(err, "problem writing test file")
	_, err = GetDB([]string{}, testFile)
	a.NotNil(err, "there must be an error when the state file is corrupt")
}

func TestSaveStateFile(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	a.Nil(os.RemoveAll(testFile2), "problem removing test file")
	sub, err := GetDB([]string{}, testFile2)
	a.Nil(err, "there must be no error creating the initial state file")
	a.Nil(sub.SaveStateFile(), "there must be no error saving the state file")
	sub, err = GetDB([]string{}, "")
	a.Nil(err, "there must be no error when the state file does not exist")
	a.Nil(sub.SaveStateFile(), "there must be no error when the state file does not exist")
}

func TestGetEvents(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub, err := GetDB([]string{}, testFile2)
	a.NotNil(sub.GetEvents(), "the events map must not be nil")
	a.Nil(err, "getting a db must produce no error")
	a.EqualValues(0, len(sub.GetEvents()), "event count must be 0 since none have been added")
	sub.UpdateEvent("event_test", nil)
	a.EqualValues(1, len(sub.GetEvents()), "event count must be 1 since 1 was added")
}

func TestGetEvent(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub, err := GetDB([]string{}, "")
	a.NotNil(sub.GetEvents(), "the events map must not be nil")
	a.Nil(err, "getting a db must produce no error")
	sub.UpdateEvent("event_test", nil)
	evn, err := sub.GetEvent("event_test")
	a.Nil(err, "there must be no error getting the events that was created")
	a.NotNil(evn, "the event rules map must not be nil")
	a.EqualValues(1, len(sub.GetEvents()), "event count must be 1 since 1 was added")
	evn, err = sub.GetEvent("missing_event")
	a.NotNil(err, "the event is missing and must produce an error")
	a.Nil(evn, "the event rules map must be nil when the event is missing")
}

func TestUpdateEvent(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	sub.UpdateEvent("event_test", nil)
	a.NotNil(sub.Events["event_test"], "the event rules map must not be nil")
	a.EqualValues(0, len(sub.Events["event_test"]), "the event rules map must have zero length")

	// Add 1 rule
	sub.UpdateEvent("event_test", map[string]string{"rule_name": "bar"})
	a.EqualValues(1, len(sub.Events["event_test"]), "the event rules map must have length of 1")
	a.EqualValues("bar", sub.Events["event_test"]["rule_name"], "the rule has the wrong value")
	// Update the same rule.
	sub.UpdateEvent("event_test", map[string]string{"rule_name": "bar2"})
	a.EqualValues(1, len(sub.Events["event_test"]), "the event rules map must have length of 1")
	a.EqualValues("bar2", sub.Events["event_test"]["rule_name"], "the rule did not update")
	// Add a enw rule.
	sub.UpdateEvent("event_test", map[string]string{"rule_name2": "some value"})
	a.EqualValues(2, len(sub.Events["event_test"]), "the event rules map must have length of 1")
	a.EqualValues("some value", sub.Events["event_test"]["rule_name2"], "the rule has the wrong value")
	// Delete a rule.
	sub.UpdateEvent("event_test", map[string]string{"rule_name": ""})
	a.EqualValues(1, len(sub.Events["event_test"]), "the event rules map must have length of 1")
	a.EqualValues("some value", sub.Events["event_test"]["rule_name2"], "the second rule has the wrong value")
}

func TestRemoveEvent(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	a.EqualValues(0, sub.RemoveEvent("no_event"), "event had no subscribers and must not produce any deletions")
	// Make two events to remove.
	sub.Events["some_event"] = nil
	sub.Events["some_event2"] = nil
	// Subscribe a user to one of them.
	s := sub.CreateSub("test_contact", "api", true, false)
	a.Nil(s.Subscribe("some_event2"))
	a.EqualValues(1, sub.RemoveEvent("some_event2"), "event had 1 subscriber")
	a.EqualValues(1, len(sub.GetEvents()), "the event must be deleted")
	a.EqualValues(0, sub.RemoveEvent("some_event"), "event had no subscribers and must not produce any deletions")
	a.EqualValues(0, len(sub.GetEvents()), "the event must be deleted")
}

func TestCreateSub(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	sub.CreateSub("myContacNameTest", "apiValueHere", true, false)
	a.EqualValues(1, len(sub.Subscribers), "there must be one subscriber")
	a.True(sub.Subscribers[0].Admin, "admin must be true")
	a.False(sub.Subscribers[0].Ignored, "ignore must be false")
	// Update values for existing contact.
	sub.CreateSub("myContacNameTest", "apiValueHere", false, true)
	a.EqualValues(1, len(sub.Subscribers), "there must still be one subscriber")
	a.False(sub.Subscribers[0].Admin, "admin must be changed to false")
	a.True(sub.Subscribers[0].Ignored, "ignore must be changed to true")
	a.True(sub.Subscribers[0].Ignored, "ignore must be changed to true")
	a.EqualValues(sub.Subscribers[0].Contact, "myContacNameTest", "contact value is incorrect")
	a.EqualValues(sub.Subscribers[0].API, "apiValueHere", "api value is incorrect")
	// Add another contact.
	sub.CreateSub("myContacName2Test", "apiValueHere", false, true)
	a.EqualValues(2, len(sub.Subscribers), "there must be two subscribers")
	a.NotNil(sub.Subscribers[1].Events, "events map must not be nil")
}

func TestGetSubscriber(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	// Test missing subscriber
	_, err := sub.GetSubscriber("im not here", "fake")
	a.EqualValues(ErrorSubscriberNotFound, err, "must have a subscriber not found error")
	// Test getting real subscriber
	sub.CreateSub("myContacNameTest", "apiValueHere", true, false)
	s, err := sub.GetSubscriber("myContacNameTest", "apiValueHere")
	a.Nil(err, "must not produce an error getting existing subscriber")
	a.EqualValues("myContacNameTest", s.GetContact(), "wrong contact value returned")
	a.EqualValues("apiValueHere", s.GetAPI(), "wrong api value returned")
	a.True(s.IsAdmin(), "wrong admin value returned")
	a.False(s.IsIgnored(), "wrong ignore value returned")
}

func TestUnSubscribe(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	// Add 1 subscriber and 3 subscriptions.
	s := sub.CreateSub("myContacNameTest", "apiValueHere", true, true)
	a.Nil(s.Subscribe("event_name"))
	a.Nil(s.Subscribe("event_name2"))
	a.Nil(s.Subscribe("event_name3"))
	// Make sure we can't add the same event twice.
	a.EqualValues(ErrorEventExists, s.Subscribe("event_name3"), "duplicate event allowed")
	// Remove a subscription.
	a.Nil(s.UnSubscribe("event_name3"))
	a.EqualValues(2, len(sub.Subscribers[0].Events), "there must be two subscriptions remaining")
	// Remove another.
	a.Nil(s.UnSubscribe("event_name2"))
	a.EqualValues(1, len(sub.Subscribers[0].Events), "there must be one subscription remaining")
	// Make sure we get accurate error when removing a missing event subscription.
	a.EqualValues(ErrorEventNotFound, s.UnSubscribe("event_name_not_here"))
}

func TestPause(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	s := sub.CreateSub("contact", "api", true, false)
	a.Nil(s.Subscribe("eventName"))
	// Make sure pausing a missing event returns the proper error.
	a.EqualValues(ErrorEventNotFound, s.Pause("fake event", 0))
	// Testing a real unpause.
	a.Nil(s.Pause("eventName", 0))
	a.WithinDuration(time.Now(), sub.Subscribers[0].Events["eventName"], 1*time.Second)
	// Testing a real pause.
	a.Nil(s.Pause("eventName", 3600*time.Second))
	a.WithinDuration(time.Now().Add(3600*time.Second), sub.Subscribers[0].Events["eventName"], 1*time.Second)
}

func TestGetSubscribers(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	sub := &subscribe{Events: make(map[string]map[string]string)}
	subs := sub.GetSubscribers("evn")
	a.EqualValues(0, len(subs), "there must be no subscribers")
	// Add 1 subscriber and 3 subscriptions.
	s := sub.CreateSub("myContacNameTest", "apiValueHere", true, false)
	a.Nil(s.Subscribe("event_name"))
	a.Nil(s.Subscribe("event_name2"))
	a.Nil(s.Subscribe("event_name3"))
	// Add 1 more subscriber and 3 more subscriptions, 2 paused.
	s = sub.CreateSub("myContacNameTest2", "apiValueHere", true, false)
	a.Nil(s.Subscribe("event_name"))
	a.Nil(s.Subscribe("event_name2"))
	a.Nil(s.Subscribe("event_name3"))
	a.Nil(s.Pause("event_name2", 10*time.Second))
	a.Nil(s.Pause("event_name3", 10*time.Minute))
	// Add another ignore subscriber with 1 subscription.
	s = sub.CreateSub("myContacNameTest3", "apiValueHere", true, true)
	a.Nil(s.Subscribe("event_name"))
	// Test that ignore keeps the ignored subscriber out.
	a.EqualValues(2, len(sub.GetSubscribers("event_name")), "there must be 2 subscribers")
	// Test that resume time keeps a subscriber out.
	a.EqualValues(1, len(sub.GetSubscribers("event_name2")), "there must be 1 subscriber")
	a.EqualValues(1, len(sub.GetSubscribers("event_name3")), "there must be 1 subscriber")
}
