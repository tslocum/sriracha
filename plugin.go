package sriracha

import (
	"fmt"
	"log"
	"reflect"
	"strings"
)

var srirachaServer *Server

type PluginConfigType int

const (
	TypeString  PluginConfigType = 0
	TypeInteger PluginConfigType = 1
	TypeFloat   PluginConfigType = 2
	TypeEnum    PluginConfigType = 3
)

type PluginConfig struct {
	Type        PluginConfigType
	Name        string
	Default     string
	Description string
	Multiple    bool
	Value       string
}

func (c *PluginConfig) Options() []string {
	if !c.Multiple {
		return []string{c.Value}
	}
	return strings.Split(c.Value, "|")
}

type Plugin interface {
	About() string
}

type PluginWithConfig interface {
	Plugin
	Config() []PluginConfig
}

type PluginWithPost interface {
	Plugin
	Post(db *Database, post *Post) error
}

func RegisterPlugin(plugin interface{}) {
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
	_ = about // TODO

	var events []string
	var config []PluginConfig

	pConfig, ok := plugin.(PluginWithConfig)
	if ok {
		config = pConfig.Config()
		for i := range config {
			config[i].Value = config[i].Default
		}
	}
	_, ok = plugin.(PluginWithPost)
	if ok {
		events = append(events, "post")
	}
	if len(events) == 0 {
		events = append(events, "none")
	}
	fmt.Printf("%s loaded. Receives: %s\n", name, strings.Join(events, ", "))

	info := &PluginInfo{
		ID:     len(allPlugins) + 1,
		Name:   name,
		About:  about,
		Events: events,
		Config: config,
	}
	allPlugins = append(allPlugins, plugin)
	allPluginInfo = append(allPluginInfo, info)
}

var allPlugins []interface{}
var allPluginInfo []*PluginInfo

type PluginInfo struct {
	ID     int
	Name   string
	About  string
	Events []string
	Config []PluginConfig
}
