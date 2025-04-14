package sriracha

import (
	"fmt"
	"log"
	"reflect"
	"strings"
)

// PluginConfigType represents the type of a plugin configuration option.
type PluginConfigType int

const (
	TypeString  PluginConfigType = 0
	TypeInteger PluginConfigType = 1
	TypeFloat   PluginConfigType = 2
	TypeEnum    PluginConfigType = 3
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

// Options returns the value of the provided option as a collection of strings.
func (c *PluginConfig) Options() []string {
	if !c.Multiple {
		return []string{c.Value}
	}
	return strings.Split(c.Value, "|")
}

// Plugin describes the required methods for any plugin.
type Plugin interface {
	About() string
}

// PluginWithConfig describes the required methods for a plugin with configuration options.
type PluginWithConfig interface {
	Plugin
	Config() []PluginConfig
}

// PluginWithPost describes the required methods for a plugin subscribing to post events.
type PluginWithPost interface {
	Plugin
	Post(db *Database, post *Post) error
}

// RegisterPlugin registers a sriracha plugin to receive any subscribed events.
// Plugins must call this function in init(). See [PluginWithConfig] and [PluginWithPost].
func RegisterPlugin(plugin any) {
	if srirachaServer == nil {
		panic("sriracha server not yet started")
	}

	v := reflect.ValueOf(plugin)
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	name := v.Type().Name()

	pAbout, ok := plugin.(Plugin)
	if !ok {
		log.Fatalf("%s does not implement required methods", name)
	}
	about := pAbout.About()

	var events []string
	var config []PluginConfig

	pConfig, ok := plugin.(PluginWithConfig)
	if ok {
		config = pConfig.Config()
		for i := range config {
			config[i].Value = config[i].Default
		}
	}

	pPost, ok := plugin.(PluginWithPost)
	if ok {
		events = append(events, "post")
		allPluginPostHandlers = append(allPluginPostHandlers, pPost.Post)
	}

	if len(events) == 0 {
		events = append(events, "none")
	}

	fmt.Printf("%s loaded. Receives: %s\n", name, strings.Join(events, ", "))

	info := &pluginInfo{
		ID:     len(allPlugins) + 1,
		Name:   name,
		About:  about,
		Config: config,
		Events: events,
	}
	allPlugins = append(allPlugins, plugin)
	allPluginInfo = append(allPluginInfo, info)
}

type pluginInfo struct {
	ID     int
	Name   string
	About  string
	Config []PluginConfig
	Events []string
}

type postHandler func(db *Database, post *Post) error

var allPlugins []any
var allPluginInfo []*pluginInfo
var allPluginPostHandlers []postHandler
