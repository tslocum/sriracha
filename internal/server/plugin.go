package server

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"maps"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"slices"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	"codeberg.org/tslocum/sriracha/plugin/bbcode/bbcode"
	"codeberg.org/tslocum/sriracha/plugin/fortune/fortune"
	"codeberg.org/tslocum/sriracha/plugin/irc/irc"
	"codeberg.org/tslocum/sriracha/plugin/password/password"
	"codeberg.org/tslocum/sriracha/plugin/robot9000/robot9000"
	"codeberg.org/tslocum/sriracha/plugin/statistics/statistics"
	"codeberg.org/tslocum/sriracha/plugin/wordfilter/wordfilter"
)

type rulesHandler func(db sriracha.DB, board *Board) (template.HTML, error)

type rulesHandlerInfo struct {
	Name    string
	Handler rulesHandler
}

type attachHandler func(db sriracha.DB, post *Post, file multipart.File) (handled bool, err error)

type attachHandlerInfo struct {
	Name    string
	Handler attachHandler
}

type embedHandler func(db sriracha.DB, post *Post, embedURL string) (handled bool, err error)

type embedHandlerInfo struct {
	Name    string
	Handler embedHandler
}

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

type serveHandler func(db sriracha.DB, a *Account, w http.ResponseWriter, r *http.Request) (template.HTML, error)

type serveHandlerInfo struct {
	Name    string
	Handler serveHandler
}

type pluginInfo struct {
	ID       int
	Name     string
	FullName string
	About    string
	Help     template.HTML
	Config   []sriracha.PluginConfig
	Events   []string
	Serve    serveHandler
}

var (
	allPlugins              []any
	allPluginInfo           []*pluginInfo
	allPluginRulesHandlers  []rulesHandlerInfo
	allPluginAttachHandlers []attachHandlerInfo
	allPluginEmbedHandlers  []embedHandlerInfo
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
	info.FullName = v.Type().Name()
	info.Name = strings.ToLower(info.FullName)

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

	if pRules, ok := plugin.(sriracha.PluginWithRules); ok {
		info.Events = append(info.Events, "Rules")
		allPluginRulesHandlers = append(allPluginRulesHandlers, rulesHandlerInfo{strings.ToLower(info.Name), pRules.Rules})
	}

	if pAttach, ok := plugin.(sriracha.PluginWithAttach); ok {
		info.Events = append(info.Events, "Attach")
		allPluginAttachHandlers = append(allPluginAttachHandlers, attachHandlerInfo{strings.ToLower(info.Name), pAttach.Attach})
	}

	if pEmbed, ok := plugin.(sriracha.PluginWithEmbed); ok {
		info.Events = append(info.Events, "Embed")
		allPluginEmbedHandlers = append(allPluginEmbedHandlers, embedHandlerInfo{strings.ToLower(info.Name), pEmbed.Embed})
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

func (s *Server) loadPlugins(disablePlugins string) error {
	// Load built-in (official) plugins.
	disable := strings.Split(disablePlugins, ",")
	var disableAll bool
	for i := range disable {
		disable[i] = strings.ToLower(strings.TrimSpace(disable[i]))
		if disable[i] == "all" {
			disableAll = true
		}
	}
	if !disableAll {
		officialPlugins := map[string]func() any{
			"bbcode": func() any {
				return bbcode.NewBBCode()
			},
			"fortune": func() any {
				return fortune.NewFortune()
			},
			"irc": func() any {
				return irc.NewIRC()
			},
			"password": func() any {
				return password.NewPassword()
			},
			"robot9000": func() any {
				return robot9000.NewRobot9000()
			},
			"statistics": func() any {
				return statistics.NewStatistics()
			},
			"wordfilter": func() any {
				return wordfilter.NewWordfilter()
			},
		}
		pluginNames := slices.Collect(maps.Keys(officialPlugins))
		slices.Sort(pluginNames)
		for _, pluginName := range pluginNames {
			if slices.Contains(disable, pluginName) {
				continue
			}
			pluginFunc := officialPlugins[pluginName]
			s.registerPlugin(pluginFunc())
		}
	}

	// Load custom plugins.
	for _, pluginPath := range flag.Args() {
		err := s.loadPlugin(pluginPath)
		if err != nil {
			return err
		}
	}

	// Print plugin information.
	if len(allPluginInfo) != 0 {
		var plural string
		if len(allPluginInfo) != 1 {
			plural = "s"
		}
		var names []string
		for _, info := range allPluginInfo {
			names = append(names, info.FullName)
		}
		fmt.Printf("Loaded %d plugin%s: %s.\n", len(allPluginInfo), plural, strings.Join(names, ", "))
	}
	return nil
}
