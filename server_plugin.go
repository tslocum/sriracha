package sriracha

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func (s *Server) servePlugin(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}
	data.Template = "manage_plugin"

	pluginID := pathInt(r, "/sriracha/plugin/reset/")
	if pluginID > 0 && pluginID <= len(allPluginInfo) {
		var changes string

		pUpdate, _ := allPlugins[pluginID-1].(PluginWithUpdate)
		info := allPluginInfo[pluginID-1]
		for i, c := range info.Config {
			defaultValue := c.Default
			if c.Type == TypeEnum {
				defaultValue = ""
			}

			if info.Config[i].Value == defaultValue {
				continue
			}
			oldValue := info.Config[i].Value

			db.SaveString(strings.ToLower(info.Name+"."+c.Name), defaultValue)
			info.Config[i].Value = defaultValue

			if pUpdate != nil {
				pluginDB := &Database{
					conn:   db.conn,
					plugin: strings.ToLower(info.Name),
				}
				pUpdate.Update(pluginDB, c.Name)
			}

			oldLabel := oldValue
			newLabel := info.Config[i].Value
			if info.Config[i].Type == TypeBoolean {
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

		if changes != "" {
			db.log(data.Account, nil, fmt.Sprintf("Reset plugin %s", info.Name), changes)
		}

		http.Redirect(w, r, fmt.Sprintf("/sriracha/plugin/%d", pluginID), http.StatusFound)
		return
	}

	pluginID = pathInt(r, "/sriracha/plugin/")
	if pluginID > 0 && pluginID <= len(allPluginInfo) {
		info := allPluginInfo[pluginID-1]
		data.Manage.Plugin = info

		if r.Method == http.MethodPost {
			r.ParseForm()
			formKeys := make([]string, len(r.Form))
			var i int
			for key := range r.Form {
				formKeys[i] = key
				i++
			}
			sort.Slice(formKeys, func(i, j int) bool {
				return formKeys[i] < formKeys[j]
			})

			pUpdate, _ := allPlugins[pluginID-1].(PluginWithUpdate)

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
								newValue += "|"
							}
							newValue += v
						}
					}
				}

				if info.Config[i].Value != newValue {
					oldLabel := info.Config[i].Value
					newLabel := newValue
					if info.Config[i].Type == TypeBoolean {
						if info.Config[i].Value != "1" {
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

					db.SaveString(strings.ToLower(info.Name+"."+c.Name), newValue)
					info.Config[i].Value = newValue

					if pUpdate != nil {
						pluginDB := &Database{
							conn:   db.conn,
							plugin: strings.ToLower(info.Name),
						}
						pUpdate.Update(pluginDB, c.Name)
					}
				}
			}

			if changes != "" {
				db.log(data.Account, nil, fmt.Sprintf("Updated plugin %s", info.Name), changes)
			}

			http.Redirect(w, r, "/sriracha/plugin", http.StatusFound)
		}
		return
	}

	data.Manage.Plugins = allPluginInfo
}
