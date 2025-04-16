package sriracha

import (
	"fmt"
	"log"
	"net/http"
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
		http.Redirect(w, r, fmt.Sprintf("/sriracha/plugin/%d", pluginID), http.StatusFound)
		return
	}

	pluginID, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/plugin/"))
	if err == nil && pluginID > 0 && pluginID <= len(allPluginInfo) {
		info := allPluginInfo[pluginID-1]
		data.Manage.Plugin = info

		if r.Method == http.MethodPost {
			for i, c := range info.Config {
				var newValue string
				for key, values := range r.Form {
					if len(values) > 0 && strings.HasPrefix(key, "config_"+c.Name) {
						if strings.TrimSpace(values[0]) == "" {
							continue
						} else if newValue != "" {
							newValue += "|"
						}
						newValue += values[0]
					}
				}
				err := db.SaveString(strings.ToLower(info.Name+"."+c.Name), newValue)
				if err != nil {
					log.Fatal(err)
				}
				info.Config[i].Value = newValue
			}
			http.Redirect(w, r, "/sriracha/plugin", http.StatusFound)
		}
		return
	}

	data.Manage.Plugins = allPluginInfo
}
