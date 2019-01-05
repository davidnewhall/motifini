package subscribe

/* Subscriptions Library!
    Reasonably Generic and fully tested. May work in your application!
		Check out the interfaces in types.go to get an idea how it works.
*/
import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// GetDB returns an interface to manage events
func GetDB(APIsEnabled []string, StateFile string) (SubDB, error) {
	s := &subscribe{
		enableAPIs:  APIsEnabled,
		stateFile:   StateFile,
		Events:      make(map[string]map[string]string),
		Subscribers: []subscriber{},
	}
	return s, s.LoadStateFile()
} /* done */

// LoadStateFile data from a json file.
func (s *subscribe) LoadStateFile() error {
	if s.stateFile == "" {
		return nil
	}
	if buf, err := ioutil.ReadFile(s.stateFile); os.IsNotExist(err) {
		return s.SaveStateFile()
	} else if err != nil {
		return err
	} else if err := json.Unmarshal(buf, s); err != nil {
		return err
	}
	return nil
} /* done */

// GetStateJSON returns the state data in json format.
func (s *subscribe) GetStateJSON() (string, error) {
	s.RLock()
	defer s.RUnlock()
	b, err := json.Marshal(s)
	return string(b), err
} /* done */

// SaveStateFile writes out the state file.
func (s *subscribe) SaveStateFile() error {
	if s.stateFile == "" {
		return nil
	}
	s.RLock()
	defer s.RUnlock()
	if buf, err := json.Marshal(s); err != nil {
		return err
	} else if err := ioutil.WriteFile(s.stateFile, buf, 0640); err != nil {
		return err
	}
	return nil
} /* done */

/************************
 *   Events Methods   *
 ************************/

// GetEvents returns all the configured events.
func (s *subscribe) GetEvents() map[string]map[string]string {
	s.RLock()
	defer s.RUnlock()
	return s.Events
} /* done */

// GetEvent returns the rules for an event.
// Rules can be used by the user for whatever they way.
func (s *subscribe) GetEvent(name string) (map[string]string, error) {
	s.RLock()
	defer s.RUnlock()
	if rules, ok := s.Events[name]; ok {
		return rules, nil
	}
	return nil, ErrorEventNotFound
} /* done */

// UpdateEvent adds or updates an event.
func (s *subscribe) UpdateEvent(name string, rules map[string]string) bool {
	s.Lock()
	defer s.Unlock()
	if rules == nil {
		rules = make(map[string]string)
	}
	if _, ok := s.Events[name]; !ok {
		s.Events[name] = rules
		return true
	}
	for ruleName, rule := range rules {
		if rule == "" {
			delete(s.Events[name], ruleName)
		} else {
			s.Events[name][ruleName] = rule
		}
	}
	return false
} /* done */

// RemoveEvent obliterates an event and all subsciptions for it.
func (s *subscribe) RemoveEvent(name string) (removed int) {
	s.Lock()
	delete(s.Events, name)
	s.Unlock()
	for i := range s.Subscribers {
		if _, ok := s.Subscribers[i].Events[name]; ok {
			s.Subscribers[i].Lock()
			delete(s.Subscribers[i].Events, name)
			s.Subscribers[i].Unlock()
			removed++
		}
	}
	return
} /* done */

/**************************
 *   Subscriber Methods   *
 **************************/

// CreateSub creates or updates a subscriber.
func (s *subscribe) CreateSub(contact, api string, admin, ignore bool) SubInterface {
	for i := range s.Subscribers {
		if contact == s.Subscribers[i].Contact && api == s.Subscribers[i].API {
			s.Subscribers[i].Admin = admin
			s.Subscribers[i].Ignored = ignore
			// Already exists, return it.
			return &s.Subscribers[i]
		}
	}

	s.Subscribers = append(s.Subscribers, subscriber{
		Contact: contact,
		API:     api,
		Admin:   admin,
		Ignored: ignore,
		Events:  make(map[string]time.Time),
	})
	return &s.Subscribers[len(s.Subscribers)-1:][0]
} /* done */

// GetSubscriber gets a subscriber based on their contact info.
func (s *subscribe) GetSubscriber(contact, api string) (SubInterface, error) {
	sub := &subscriber{}
	for i := range s.Subscribers {
		if s.Subscribers[i].Contact == contact && s.Subscribers[i].API == api {
			return &s.Subscribers[i], nil
		}
	}
	return sub, ErrorSubscriberNotFound
} /* done */

// GetAdmins returns a list of subscribed admins.
func (s *subscribe) GetAdmins() (subs []SubInterface) {
	for i := range s.Subscribers {
		if s.Subscribers[i].Admin {
			subs = append(subs, &s.Subscribers[i])
		}
	}
	return
}

// GetIgnored returns a list of ignored subscribers.
func (s *subscribe) GetIgnored() (subs []SubInterface) {
	for i := range s.Subscribers {
		if s.Subscribers[i].Ignored {
			subs = append(subs, &s.Subscribers[i])
		}
	}
	return
}

// GetIgnored returns a list of ignored subscribers.
func (s *subscribe) GetAllSubscribers() (subs []SubInterface) {
	for i := range s.Subscribers {
		subs = append(subs, &s.Subscribers[i])
	}
	return
}

// Ignore a subscriber.
func (s *subscriber) Ignore() {
	s.Ignored = true
}

// MakeAdmin a subscriber.
func (s *subscriber) MakeAdmin() {
	s.Admin = true
}

// Unignore a subscriber.
func (s *subscriber) Unignore() {
	s.Ignored = false
}

// Unadmin a subscriber.
func (s *subscriber) Unadmin() {
	s.Admin = false
}

// IsIgnored returns ignored status of a sub.
func (s *subscriber) IsIgnored() bool {
	return s.Ignored
}

// IsAdmin returns admin status of a sub.
func (s *subscriber) IsAdmin() bool {
	return s.Admin
}

// GetAPI returns a contact's API.
func (s *subscriber) GetAPI() string {
	return s.API
}

// GetContact returns a contact's contact value.
func (s *subscriber) GetContact() string {
	return s.Contact
}

/****************************
 *   Subscription Methods   *
 ****************************/

// Subscribe adds an event subscription to a subscriber.
func (s *subscriber) Subscribe(eventName string) error {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.Events[eventName]; ok {
		return ErrorEventExists
	}
	s.Events[eventName] = time.Now()
	return nil
}

// UnSubscribe a subscriber from a event subscription.
func (s *subscriber) UnSubscribe(eventName string) error {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.Events[eventName]; !ok {
		return ErrorEventNotFound
	}
	delete(s.Events, eventName)
	return nil
}

// Pause (or unpause with 0 duration) a subscriber's event subscription.
func (s *subscriber) Pause(eventName string, duration time.Duration) error {
	s.Lock()
	defer s.Unlock()
	_, ok := s.Events[eventName]
	if !ok {
		return ErrorEventNotFound
	}
	s.Events[eventName] = time.Now().Add(duration)
	return nil
}

// EventNames returns a subscriber's event names.
func (s *subscriber) Subscriptions() (events map[string]time.Time) {
	s.Lock()
	defer s.Unlock()
	events = s.Events
	return
}

// GetSubscribers returns a list of valid event subscribers.
func (s *subscribe) GetSubscribers(eventName string) (subscribers []SubInterface) {
	for i := range s.Subscribers {
		if s.Subscribers[i].Ignored {
			continue
		}
		for event, evnData := range s.Subscribers[i].Events {
			if event == eventName && evnData.Before(time.Now()) && checkAPI(s.Subscribers[i].API, s.enableAPIs) {
				subscribers = append(subscribers, &s.Subscribers[i])
			}
		}
	}
	return
} /* done */

// checkAPI just looks for a string in a slice of strings with a twist.
func checkAPI(s string, slice []string) bool {
	if len(slice) < 1 {
		return true
	}
	for _, v := range slice {
		if v == s || strings.HasPrefix(s, v) || v == "all" || v == "any" {
			return true
		}
	}
	return false
} /* done */
