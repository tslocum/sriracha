package sriracha

import (
	"fmt"
	"log"
	"reflect"
	"strings"
)

var srirachaServer *Server

type Plugin interface {
	About() string
}

type PluginWithPost interface {
	Plugin
	Post(db *Database, post *Post) error
}

func RegisterPlugin(plugin interface{}) {
	if srirachaServer == nil {
		panic("sriracha server not yet started")
	}

	p, ok := plugin.(Plugin)
	if !ok {
		log.Fatal("plugin does not implement required methods")
	}

	var events []string

	_, ok = plugin.(PluginWithPost)
	if ok {
		events = append(events, "post")
	}
	if len(events) == 0 {
		events = append(events, "none")
	}
	fmt.Printf("%s loaded. Receives: %s\n", reflect.ValueOf(p).Elem().Type().Name(), strings.Join(events, ", "))
}
