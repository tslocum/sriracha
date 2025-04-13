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
}

type Plugin interface {
	About() string
}

type PluginWithConfig interface {
	Plugin
	Config() []*PluginConfig
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

	pConfig, ok := plugin.(PluginWithConfig)
	if ok {
		log.Println("CONFIG", pConfig.Config())
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
		Name:   name,
		About:  about,
		Events: events,
	}
	allPlugins = append(allPlugins, plugin)
	allPluginInfo = append(allPluginInfo, info)
}

var allPlugins []interface{}
var allPluginInfo []*PluginInfo

type PluginInfo struct {
	Name   string
	About  string
	Events []string
}
