package subscribe

import (
	"sync"
	"time"
)

// Error enables constant errors.
type Error string

// Error allows a string to satisfy the error type.
func (e Error) Error() string {
	return string(e)
}

// ErrorSubIDNotFound and the rest are error strings.
const (
	ErrorSubscriberNotFound = Error("subscriber not found")
	ErrorEventNotFound      = Error("event not found")
	ErrorEventExists        = Error("event already exists")
)

// subscriber describes the contact info and subscriptions for a person.
type subscriber struct {
	API          string               `json:"api"`
	Contact      string               `json:"contact"`
	Events       map[string]time.Time `json:"events"`
	Admin        bool                 `json:"is_admin"`
	Ignored      bool                 `json:"ignored"`
	sync.RWMutex                      // Locks subs/events maps
}

// subscribe is the data needed to initialize this module.
type subscribe struct {
	enableAPIs   []string                     // imessage, skype, pushover, email, slack, growl, all, any
	stateFile    string                       // like: /usr/local/var/lib/motifini/subscribers.json
	Events       map[string]map[string]string `json:"events"`
	Subscribers  []subscriber                 `json:"subscribers"`
	sync.RWMutex                              // Locks events
}

// SubDB allows us to mock this library when doing external testing.
type SubDB interface {
	GetEvents() map[string]map[string]string
	UpdateEvent(eventName string, rules map[string]string) (count bool)
	RemoveEvent(eventName string) (removed int)
	GetEvent(name string) (rules map[string]string, err error)
	CreateSub(contact, api string, admin, ignore bool) (subscriber SubInterface)
	GetSubscriber(contact, api string) (subscriber SubInterface, err error)
	GetAdmins() (subscribers []SubInterface)
	GetIgnored() (subscribers []SubInterface)
	GetAllSubscribers() (subscribers []SubInterface)
	GetSubscribers(eventName string) (subscribers []SubInterface)
	SaveStateFile() error
	GetStateJSON() (string, error)
}

// SubInterface allows mocks to methods against a subscriber.
type SubInterface interface {
	GetAPI() string
	GetContact() string
	Ignore()
	Unignore()
	IsIgnored() bool
	IsAdmin() bool
	MakeAdmin()
	Unadmin()
	Subscriptions() (events map[string]time.Time)
	Subscribe(string) error
	UnSubscribe(string) error
	Pause(string, time.Duration) error
}
