package sriracha

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func (s *Server) servePlugin(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_plugin"

	pluginID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/plugin/reset/"))
	if err == nil && pluginID > 0 && pluginID <= len(allPluginInfo) {
		info := allPluginInfo[pluginID-1]
		for i, c := range info.Config {
			err := db.SaveString(strings.ToLower(info.Name+"."+c.Name), c.Default)
			if err != nil {
				log.Fatal(err)
			}
			info.Config[i].Value = c.Default
		}

		db.log(data.Account, nil, fmt.Sprintf("Reset plugin %s", info.Name), "")

		http.Redirect(w, r, fmt.Sprintf("/sriracha/plugin/%d", pluginID), http.StatusFound)
		return
	}

	pluginID, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/plugin/"))
	if err == nil && pluginID > 0 && pluginID <= len(allPluginInfo) {
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

			var changes string
			for i, c := range info.Config {
				var newValue string
				for _, key := range formKeys {
					values := r.Form[key]
					if len(values) > 0 && strings.HasPrefix(key, "config_"+c.Name) {
						if strings.TrimSpace(values[0]) == "" {
							continue
						} else if newValue != "" {
							newValue += "|"
						}
						newValue += values[0]
					}
				}

				if info.Config[i].Value != newValue {
					changes += fmt.Sprintf(` (%s: "%s" -> "%s")`, strings.Title(c.Name), info.Config[i].Value, newValue)

					err := db.SaveString(strings.ToLower(info.Name+"."+c.Name), newValue)
					if err != nil {
						log.Fatal(err)
					}
					info.Config[i].Value = newValue
				}
			}

			db.log(data.Account, nil, fmt.Sprintf("Updated plugin %s", info.Name), changes)

			http.Redirect(w, r, "/sriracha/plugin", http.StatusFound)
		}
		return
	}

	data.Manage.Plugins = allPluginInfo
}
