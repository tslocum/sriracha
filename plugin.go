package sriracha

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

// PluginConfigType represents the type of a plugin configuration option.
type PluginConfigType int

// Plugin config types.
const (
	TypeBoolean PluginConfigType = 0
	TypeInteger PluginConfigType = 1
	TypeFloat   PluginConfigType = 2
	TypeEnum    PluginConfigType = 3
	TypeString  PluginConfigType = 4
	TypeBoard   PluginConfigType = 5
)

// PluginConfig represents a plugin configuration option.
type PluginConfig struct {
	Type      PluginConfigType
	Multiple  bool
	Name      string
	Info      string
	Value     string
	Default   string
	Sensitive bool // Sensitive options are excluded from the audit log.
}

func (c PluginConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.Name) == "":
		return fmt.Errorf("name must be set")
	case c.Type < TypeBoolean || c.Type > TypeBoard:
		return fmt.Errorf("invalid type")
	case c.Type == TypeBoolean && c.Multiple:
		return fmt.Errorf("multi-value boolean options are not allowed")
	default:
		return nil
	}
}

// Options returns the options of the provided configuration option as a collection of strings.
func (c PluginConfig) Options() []string {
	if c.Type != TypeEnum {
		return nil
	}
	return strings.Split(c.Default, "|||")
}

// Values returns the value of the provided configuration option as a collection of strings.
func (c PluginConfig) Values() []string {
	if c.Value == "" {
		return nil
	} else if !c.Multiple {
		return []string{c.Value}
	}
	return strings.Split(c.Value, "|||")
}

// HaveInt returns whether an integer value is selected.
func (c PluginConfig) HaveInt(i int) bool {
	for _, v := range c.Values() {
		if ParseInt(v) == i {
			return true
		}
	}
	return false
}

// Plugin describes the required methods for a plugin.
type Plugin interface {
	// About returns the plugin description.
	About() string
}

// PluginWithHelp describes the required methods for a plugin with help text.
type PluginWithHelp interface {
	// Help returns the text displayed above the available configuration options.
	Help() template.HTML
}

// PluginWithConfig describes the required methods for a plugin with configuration options.
type PluginWithConfig interface {
	Plugin

	// Config returns the available configuration options.
	Config() []PluginConfig
}

// PluginWithUpdate describes the required methods for a plugin subscribing to configuration updates.
type PluginWithUpdate interface {
	Plugin

	// Update events are sent when a configuration option is modified. Update events
	// are also sent for each configuration option when the server initializes.
	Update(db DB, key string) error
}

// PluginWithAttach describes the required methods for a plugin subscribing to attach events.
type PluginWithAttach interface {
	Plugin

	// Attach events are sent when a file is attached to a post. FileOriginal contains
	// the original file name and FileMIME contains the detected MIME type. When a
	// file attachment is handled, return true to stop propagating events.
	Attach(db DB, post *Post, file []byte) (handled bool, err error)
}

// PluginWithPost describes the required methods for a plugin subscribing to post events.
type PluginWithPost interface {
	Plugin

	// Post events are sent when a new post is being created. Message is the
	// only HTML-escaped field. Newlines are conveted into line break tags
	// after all plugins have finished processing the post.
	Post(db DB, post *Post) error
}

// PluginWithInsert describes the required methods for a plugin subscribing to insert events.
type PluginWithInsert interface {
	Plugin

	// Insert events are sent after Post events have been processed, before a
	// new post is inserted. The post may not be modified during this event.
	// Modify new posts during a Post event instead. Return an error to cancel
	// the post, or nil to continue processing.
	Insert(db DB, post *Post) error
}

// PluginWithCreate describes the required methods for a plugin subscribing to create events.
type PluginWithCreate interface {
	Plugin

	// Create events are sent when a new post is created and inserted into the
	// database, after Post and Insert events have been processed. The post may
	// not be modified during this event. Modify posts during a Post event instead.
	Create(db DB, post *Post) error
}

// PluginWithReport describes the required methods for a plugin subscribing to report events.
type PluginWithReport interface {
	Plugin

	// Report events are sent when a post is reported.
	Report(db DB, post *Post) error
}

// PluginWithAudit describes the required methods for a plugin subscribing to audit events.
type PluginWithAudit interface {
	Plugin

	// Audit events are sent when a new message is added to the audit log.
	// Based on the source of the event, user is "system", "admin" or "mod".
	Audit(db DB, user string, action string, info string) error
}

// PluginWithServe describes the required methods for a plugin with a web interface.
type PluginWithServe interface {
	Plugin

	// Serve handles plugin web requests. Only administrators and super-administrators
	// may access this page. When serving HTML responses, return the HTML and a
	// nil error. When serving any other content type, set the Conent-Type header,
	// write to the http.ResponseWriter directly and return a blank string.
	Serve(db DB, a *Account, w http.ResponseWriter, r *http.Request) (template.HTML, error)
}
