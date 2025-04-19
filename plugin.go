package sriracha

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
)

// PluginConfigType represents the type of a plugin configuration option.
type PluginConfigType int

const (
	TypeBoolean PluginConfigType = 0
	TypeInteger PluginConfigType = 1
	TypeFloat   PluginConfigType = 2
	TypeRange   PluginConfigType = 3
	TypeEnum    PluginConfigType = 4
	TypeString  PluginConfigType = 5
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
	case c.Type < TypeBoolean || c.Type > TypeString:
		return fmt.Errorf("invalid type")
	case c.Type == TypeBoolean && c.Multiple:
		return fmt.Errorf("multi-value boolean options are not allowed")
	default:
		return nil
	}
}

// Options returns the value of the provided option as a collection of strings.
func (c PluginConfig) Options() []string {
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

// PluginWithUpdate describes the required methods for a plugin subscribing to configuration updates.
type PluginWithUpdate interface {
	Plugin
	Update(db *Database, key string) error
}

// PluginWithPost describes the required methods for a plugin subscribing to post events.
type PluginWithPost interface {
	Plugin
	Post(db *Database, post *Post) error
}

// RegisterPlugin registers a Sriracha plugin to receive any subscribed events.
// Plugins must call this function in init(). See [PluginWithConfig] and [PluginWithPost].
func RegisterPlugin(plugin any) {
	if srirachaServer == nil {
		panic("Sriracha server not yet started")
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

	pUpdate, ok := plugin.(PluginWithUpdate)
	if ok {
		events = append(events, "Update")
	}

	conn, err := srirachaServer.dbPool.Acquire(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		log.Fatalf("failed to begin transaction: %s", err)
	}

	pConfig, ok := plugin.(PluginWithConfig)
	if ok {
		config = pConfig.Config()
		for i := range config {
			err := config[i].validate()
			if err != nil {
				optionName := config[i].Name
				if strings.TrimSpace(optionName) == "" {
					optionName = fmt.Sprintf("#%d", i)
				} else {
					optionName = fmt.Sprintf(`"%s"`, optionName)
				}
				log.Fatalf("%s configuration option %s is invalid: %s", name, optionName, err)
			} else if config[i].Type == TypeBoolean && config[i].Default == "" {
				config[i].Default = "0"
			}
			config[i].Value = config[i].Default

			if pUpdate != nil {
				db := &Database{
					conn:   conn,
					plugin: strings.ToLower(name),
				}
				pUpdate.Update(db, config[i].Name)
			}
		}
	}

	pPost, ok := plugin.(PluginWithPost)
	if ok {
		events = append(events, "Post")
		allPluginPostHandlers = append(allPluginPostHandlers, pPost.Post)
	}

	if len(events) == 0 {
		events = append(events, "None")
	}

	fmt.Printf("%s loaded. Events: %s\n", name, strings.Join(events, ", "))

	info := &pluginInfo{
		ID:     len(allPlugins) + 1,
		Name:   name,
		About:  about,
		Config: config,
		Events: events,
	}
	allPlugins = append(allPlugins, plugin)
	allPluginInfo = append(allPluginInfo, info)

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		log.Fatalf("failed to commit transaction: %s", err)
	}
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
