package server

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) servePlugin(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}
	data.Template = "manage_plugin"
	data.Boards = db.AllBoards()

	plugin, info := pluginByName(PathString(r, "/sriracha/plugin/reset/"))
	if plugin != nil {
		var changes string
		pUpdate, _ := plugin.(sriracha.PluginWithUpdate)
		for i, c := range info.Config {
			defaultValue := c.Default
			if c.Type == sriracha.TypeEnum {
				defaultValue = ""
			}

			oldValue := info.Config[i].Value
			if oldValue == defaultValue {
				continue
			}
			db.SaveString(strings.ToLower(info.Name+"."+c.Name), defaultValue)
			info.Config[i].Value = defaultValue

			if pUpdate != nil {
				db.SetPlugin(info.Name)
				pUpdate.Update(db, c.Name)
				db.SetPlugin("")
			}

			if c.Sensitive {
				continue
			}
			oldLabel := oldValue
			newLabel := info.Config[i].Value
			if info.Config[i].Type == sriracha.TypeBoolean {
				if oldValue != "1" {
					oldLabel = "false"
				} else {
					oldLabel = "true"
				}
				if info.Config[i].Value != "1" {
					newLabel = "false"
				} else {
					newLabel = "true"
				}
			}
			if changes != "" {
				changes += " "
			}
			changes += fmt.Sprintf(`[%s: "%s" > "%s"]`, strings.Title(strings.ReplaceAll(c.Name, "_", " ")), oldLabel, newLabel)
		}

		if _, ok := plugin.(sriracha.PluginWithRules); ok {
			s.refreshRulesCache(db)
		}

		if changes != "" {
			s.log(db, data.Account, nil, fmt.Sprintf("Reset plugin %s", info.Name), changes)
		}
		data.Redirect(w, r, fmt.Sprintf("/sriracha/plugin/%s", strings.ToLower(info.Name)))
		return
	}

	split := strings.Split(PathString(r, "/sriracha/plugin/view/"), "/")
	if len(split) > 0 {
		plugin, info = pluginByName(split[0])
		if plugin != nil {
			pServe, ok := plugin.(sriracha.PluginWithServe)
			if !ok {
				data.Redirect(w, r, "/sriracha/plugin/")
				return
			}
			msg, err := pServe.Serve(db, data.Account, w, r)
			if err != nil {
				data.ManageError(err.Error())
				return
			} else if msg != "" {
				data.Template = "manage_info"
				data.Message = template.HTML(`<h2 class="managetitle">`+strings.Title(info.Name)+`</h2>`) + msg
			} else {
				data.Template = ""
			}
			return
		}
	}

	plugin, info = pluginByName(PathString(r, "/sriracha/plugin/"))
	if plugin != nil {
		data.Manage.Plugin = info

		if r.Method == http.MethodPost {
			r.ParseForm()
			formKeys := make([]string, len(r.Form))
			{
				var i int
				for key := range r.Form {
					formKeys[i] = key
					i++
				}
				sort.Slice(formKeys, func(i, j int) bool {
					return formKeys[i] < formKeys[j]
				})
			}

			pUpdate, _ := plugin.(sriracha.PluginWithUpdate)
			var changed bool
			var changes string
			for i, c := range info.Config {
				var newValue string
				for _, key := range formKeys {
					values := r.Form[key]
					if strings.HasPrefix(key, "config_"+c.Name) && len(values) > 0 {
						for _, v := range values {
							if strings.TrimSpace(v) == "" {
								continue
							} else if newValue != "" {
								newValue += "|||"
							}
							newValue += v
						}
					}
				}

				oldValue := c.Value
				if oldValue == newValue {
					continue
				}
				db.SaveString(strings.ToLower(info.Name+"."+c.Name), newValue)
				info.Config[i].Value = newValue
				changed = true

				if pUpdate != nil {
					db.SetPlugin(info.Name)
					pUpdate.Update(db, c.Name)
					db.SetPlugin("")
				}

				if c.Sensitive {
					continue
				}
				oldLabel := oldValue
				newLabel := newValue
				if c.Type == sriracha.TypeBoolean {
					if oldValue != "1" {
						oldLabel = "false"
					} else {
						oldLabel = "true"
					}
					if newValue != "1" {
						newLabel = "false"
					} else {
						newLabel = "true"
					}
				}
				if changes != "" {
					changes += " "
				}
				changes += fmt.Sprintf(`[%s: "%s" > "%s"]`, strings.Title(strings.ReplaceAll(c.Name, "_", " ")), oldLabel, newLabel)
			}

			if _, ok := plugin.(sriracha.PluginWithRules); ok {
				s.refreshRulesCache(db)
			}

			if changed {
				s.log(db, data.Account, nil, fmt.Sprintf("Updated plugin %s", info.Name), changes)
			}
			data.Redirect(w, r, "/sriracha/plugin")
		}
		return
	}

	data.Manage.Plugins = allPluginInfo
}
