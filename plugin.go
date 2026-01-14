package sriracha

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"
	"strings"
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
	Type        PluginConfigType
	Multiple    bool
	Name        string
	Default     string
	Description string
	Value       string
}

func (c PluginConfig) validate() error {
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
		if parseInt(v) == i {
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
	Update(db *Database, key string) error
}

// PluginWithPost describes the required methods for a plugin subscribing to post events.
type PluginWithPost interface {
	Plugin

	// Post events are sent when a new post is being created. Message is the
	// only HTML-escaped field. Newlines are conveted into line break tags
	// after all plugins have finished processing the post.
	Post(db *Database, post *Post) error
}

// PluginWithInsert describes the required methods for a plugin subscribing to insert events.
type PluginWithInsert interface {
	Plugin

	// Insert events are sent after Post events have been processed, before a
	// new post is inserted. The post may not be modified during this event.
	// Modify new posts during a Post event instead. Return an error to cancel
	// the post, or nil to continue processing.
	Insert(db *Database, post *Post) error
}

// PluginWithCreate describes the required methods for a plugin subscribing to create events.
type PluginWithCreate interface {
	Plugin

	// Create events are sent when a new post is created and inserted into the
	// database, after Post and Insert events have been processed. The post may
	// not be modified during this event. Modify posts during a Post event instead.
	Create(db *Database, post *Post) error
}

// PluginWithReport describes the required methods for a plugin subscribing to report events.
type PluginWithReport interface {
	Plugin

	// Report events are sent when a post is reported.
	Report(db *Database, post *Post) error
}

// PluginWithAudit describes the required methods for a plugin subscribing to audit events.
type PluginWithAudit interface {
	Plugin

	// Audit events are sent when a new message is added to the audit log.
	// Based on the source of the event, user is "system", "admin" or "mod".
	Audit(db *Database, user string, action string, info string) error
}

// PluginWithServe describes the required methods for a plugin with a web interface.
type PluginWithServe interface {
	Plugin

	// Serve handles plugin web requests. Only administrators and super-administrators
	// may access this page. When serving HTML responses, return the HTML and a
	// nil error. When serving any other content type, set the Conent-Type header,
	// write to the http.ResponseWriter directly and return a blank string.
	Serve(db *Database, a *Account, w http.ResponseWriter, r *http.Request) (string, error)
}

// RegisterPlugin registers a Sriracha plugin to receive any subscribed events.
// Plugins must call this function in init(). See [PluginWithConfig],
// [PluginWithUpdate], [PluginWithInsert], etc. for available events.
func RegisterPlugin(plugin any) {
	if srirachaServer == nil {
		panic("Sriracha server not yet started")
	}

	info := &pluginInfo{
		ID: len(allPlugins) + 1,
	}

	v := reflect.ValueOf(plugin)
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	info.Name = v.Type().Name()

	if pAbout, ok := plugin.(Plugin); ok {
		info.About = pAbout.About()
	} else {
		log.Fatalf("%s does not implement required methods", info.Name)
	}

	if pHelp, ok := plugin.(PluginWithHelp); ok {
		info.Help = pHelp.Help()
	}

	if pConfig, ok := plugin.(PluginWithConfig); ok {
		config := pConfig.Config()
		for i := range config {
			err := config[i].validate()
			if err != nil {
				optionName := config[i].Name
				if strings.TrimSpace(optionName) == "" {
					optionName = fmt.Sprintf("#%d", i)
				} else {
					optionName = fmt.Sprintf(`"%s"`, optionName)
				}
				log.Fatalf("%s configuration option %s is invalid: %s", info.Name, optionName, err)
			} else if config[i].Type == TypeBoolean && config[i].Default == "" {
				config[i].Default = "0"
			}

			if config[i].Type == TypeEnum {
				config[i].Value = ""
			} else {
				config[i].Value = config[i].Default
			}
		}
		info.Config = config
	}

	if _, ok := plugin.(PluginWithUpdate); ok {
		info.Events = append(info.Events, "Update")
	}

	if pPost, ok := plugin.(PluginWithPost); ok {
		info.Events = append(info.Events, "Post")
		allPluginPostHandlers = append(allPluginPostHandlers, postHandlerInfo{strings.ToLower(info.Name), pPost.Post})
	}

	if pInsert, ok := plugin.(PluginWithInsert); ok {
		info.Events = append(info.Events, "Insert")
		allPluginInsertHandlers = append(allPluginInsertHandlers, insertHandlerInfo{strings.ToLower(info.Name), pInsert.Insert})
	}

	if pCreate, ok := plugin.(PluginWithCreate); ok {
		info.Events = append(info.Events, "Create")
		allPluginCreateHandlers = append(allPluginCreateHandlers, createHandlerInfo{strings.ToLower(info.Name), pCreate.Create})
	}

	if pReport, ok := plugin.(PluginWithReport); ok {
		info.Events = append(info.Events, "Report")
		allPluginReportHandlers = append(allPluginReportHandlers, reportHandlerInfo{strings.ToLower(info.Name), pReport.Report})
	}

	if pAudit, ok := plugin.(PluginWithAudit); ok {
		info.Events = append(info.Events, "Audit")
		allPluginAuditHandlers = append(allPluginAuditHandlers, auditHandlerInfo{strings.ToLower(info.Name), pAudit.Audit})
	}

	if pServe, ok := plugin.(PluginWithServe); ok {
		info.Events = append(info.Events, "Serve")
		info.Serve = pServe.Serve
		allPluginServeHandlers = append(allPluginServeHandlers, serveHandlerInfo{strings.ToLower(info.Name), pServe.Serve})
	}

	if len(info.Events) == 0 {
		info.Events = append(info.Events, "None")
	}

	fmt.Printf("%s loaded. Events: %s\n", info.Name, strings.Join(info.Events, ", "))

	allPlugins = append(allPlugins, plugin)
	allPluginInfo = append(allPluginInfo, info)
}

type postHandler func(db *Database, post *Post) error

type postHandlerInfo struct {
	Name    string
	Handler postHandler
}

type insertHandler func(db *Database, post *Post) error

type insertHandlerInfo struct {
	Name    string
	Handler insertHandler
}

type createHandler func(db *Database, post *Post) error

type createHandlerInfo struct {
	Name    string
	Handler createHandler
}

type reportHandler func(db *Database, post *Post) error

type reportHandlerInfo struct {
	Name    string
	Handler reportHandler
}

type auditHandler func(db *Database, user string, action string, info string) error

type auditHandlerInfo struct {
	Name    string
	Handler auditHandler
}

type serveHandler func(db *Database, a *Account, w http.ResponseWriter, r *http.Request) (string, error)

type serveHandlerInfo struct {
	Name    string
	Handler serveHandler
}

type pluginInfo struct {
	ID     int
	Name   string
	About  string
	Help   template.HTML
	Config []PluginConfig
	Events []string
	Serve  serveHandler
}

var (
	allPlugins              []any
	allPluginInfo           []*pluginInfo
	allPluginPostHandlers   []postHandlerInfo
	allPluginInsertHandlers []insertHandlerInfo
	allPluginCreateHandlers []createHandlerInfo
	allPluginReportHandlers []reportHandlerInfo
	allPluginAuditHandlers  []auditHandlerInfo
	allPluginServeHandlers  []serveHandlerInfo
)
