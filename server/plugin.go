package server

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
)

type postHandler func(db sriracha.DB, post *Post) error

type postHandlerInfo struct {
	Name    string
	Handler postHandler
}

type insertHandler func(db sriracha.DB, post *Post) error

type insertHandlerInfo struct {
	Name    string
	Handler insertHandler
}

type createHandler func(db sriracha.DB, post *Post) error

type createHandlerInfo struct {
	Name    string
	Handler createHandler
}

type reportHandler func(db sriracha.DB, post *Post) error

type reportHandlerInfo struct {
	Name    string
	Handler reportHandler
}

type auditHandler func(db sriracha.DB, user string, action string, info string) error

type auditHandlerInfo struct {
	Name    string
	Handler auditHandler
}

type serveHandler func(db sriracha.DB, a *Account, w http.ResponseWriter, r *http.Request) (string, error)

type serveHandlerInfo struct {
	Name    string
	Handler serveHandler
}

type pluginInfo struct {
	ID     int
	Name   string
	About  string
	Help   template.HTML
	Config []sriracha.PluginConfig
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

// registerPlugin registers a Sriracha plugin to start receiving events.
func (s *Server) registerPlugin(plugin any) {
	info := &pluginInfo{
		ID: len(allPlugins) + 1,
	}

	v := reflect.ValueOf(plugin)
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	info.Name = v.Type().Name()

	if pAbout, ok := plugin.(sriracha.Plugin); ok {
		info.About = pAbout.About()
	} else {
		log.Fatalf("%s does not implement required methods", info.Name)
	}

	if pHelp, ok := plugin.(sriracha.PluginWithHelp); ok {
		info.Help = pHelp.Help()
	}

	if pConfig, ok := plugin.(sriracha.PluginWithConfig); ok {
		config := pConfig.Config()
		for i := range config {
			err := config[i].Validate()
			if err != nil {
				optionName := config[i].Name
				if strings.TrimSpace(optionName) == "" {
					optionName = fmt.Sprintf("#%d", i)
				} else {
					optionName = fmt.Sprintf(`"%s"`, optionName)
				}
				log.Fatalf("%s configuration option %s is invalid: %s", info.Name, optionName, err)
			} else if config[i].Type == sriracha.TypeBoolean && config[i].Default == "" {
				config[i].Default = "0"
			}

			if config[i].Type == sriracha.TypeEnum {
				config[i].Value = ""
			} else {
				config[i].Value = config[i].Default
			}
		}
		info.Config = config
	}

	if _, ok := plugin.(sriracha.PluginWithUpdate); ok {
		info.Events = append(info.Events, "Update")
	}

	if pPost, ok := plugin.(sriracha.PluginWithPost); ok {
		info.Events = append(info.Events, "Post")
		allPluginPostHandlers = append(allPluginPostHandlers, postHandlerInfo{strings.ToLower(info.Name), pPost.Post})
	}

	if pInsert, ok := plugin.(sriracha.PluginWithInsert); ok {
		info.Events = append(info.Events, "Insert")
		allPluginInsertHandlers = append(allPluginInsertHandlers, insertHandlerInfo{strings.ToLower(info.Name), pInsert.Insert})
	}

	if pCreate, ok := plugin.(sriracha.PluginWithCreate); ok {
		info.Events = append(info.Events, "Create")
		allPluginCreateHandlers = append(allPluginCreateHandlers, createHandlerInfo{strings.ToLower(info.Name), pCreate.Create})
	}

	if pReport, ok := plugin.(sriracha.PluginWithReport); ok {
		info.Events = append(info.Events, "Report")
		allPluginReportHandlers = append(allPluginReportHandlers, reportHandlerInfo{strings.ToLower(info.Name), pReport.Report})
	}

	if pAudit, ok := plugin.(sriracha.PluginWithAudit); ok {
		info.Events = append(info.Events, "Audit")
		allPluginAuditHandlers = append(allPluginAuditHandlers, auditHandlerInfo{strings.ToLower(info.Name), pAudit.Audit})
	}

	if pServe, ok := plugin.(sriracha.PluginWithServe); ok {
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

func (s *Server) loadPlugin(pluginPath string) error {
	wrapErr := func(err error) error {
		return fmt.Errorf("failed to load plugin %s: %s", pluginPath, err)
	}

	info, err := os.Stat(pluginPath)
	if err != nil {
		return wrapErr(err)
	} else if info.IsDir() {
		return filepath.WalkDir(pluginPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			} else if d.IsDir() || path == pluginPath {
				return nil
			}
			return s.loadPlugin(path)
		})
	} else if !strings.HasSuffix(pluginPath, ".so") {
		return nil
	}

	const pluginExample = "plugins must declare a function named \"Plugin\" which returns a new instance:\n  func Plugin() any {\n    return &MyPlugin{}\n  }"
	plugin, err := plugin.Open(pluginPath)
	if err != nil {
		return wrapErr(err)
	}
	pluginSymbol, err := plugin.Lookup("Plugin")
	if err != nil {
		return wrapErr(fmt.Errorf("expected function \"Plugin\" was not found: " + pluginExample))
	}
	pluginFunc, ok := pluginSymbol.(func() any)
	if !ok {
		return wrapErr(fmt.Errorf("symbol \"Plugin\" was found but does not match the expected function signature: " + pluginExample))
	}
	s.registerPlugin(pluginFunc())
	return nil
}

func (s *Server) loadPlugins() error {
	for _, pluginPath := range flag.Args() {
		err := s.loadPlugin(pluginPath)
		if err != nil {
			return err
		}
	}
	return nil
}
