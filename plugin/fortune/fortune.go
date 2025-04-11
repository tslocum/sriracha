package main

import (
	"math/rand"
	"strings"

	"codeberg.org/tslocum/sriracha"
)

const (
	configTriggers = "triggers"
	configFortunes = "fortunes"
)

type Fortune struct{}

func (f *Fortune) About() string {
	return "Give your posters some good luck (or bad)."
}

func (f *Fortune) Config() []*sriracha.PluginConfig {
	var config []*sriracha.PluginConfig
	config = append(config, &sriracha.PluginConfig{
		Type:        sriracha.TypeString,
		Name:        configTriggers,
		Default:     defaultTrigger,
		Description: "The text users may input in the name or email field to receive a fortune.",
		Multiple:    true,
	})
	config = append(config, &sriracha.PluginConfig{
		Type:        sriracha.TypeString,
		Name:        configFortunes,
		Default:     defaultFortunes,
		Description: "The fortunes users may receive.",
		Multiple:    true,
	})
	return config
}

func (f *Fortune) Post(db *sriracha.Database, post *sriracha.Post) error {
	triggers, err := db.GetMultiString(configTriggers)
	if err != nil {
		return err
	}
	var showFortune bool
	for _, trigger := range triggers {
		if strings.EqualFold(post.Name, trigger) {
			post.Name = ""
			showFortune = true
		}
		if strings.EqualFold(post.Email, trigger) {
			post.Email = ""
			showFortune = true
		}
	}
	if showFortune {
		fortunes, err := db.GetMultiString(configFortunes)
		if err != nil {
			return err
		}
		fortune := fortunes[rand.Intn(len(fortunes))]
		if len(strings.TrimSpace(post.Message)) == 0 {
			post.Message = fortune
		} else {
			post.Message = fortune + "\n\n" + post.Message
		}
	}
	return nil
}

func init() {
	sriracha.RegisterPlugin(&Fortune{})
}

var _ sriracha.Plugin = &Fortune{}

const defaultTrigger = "#fortune"

const defaultFortunes = `<font color="#B604A2"><b>Your fortune: Godly Luck</b></font>|<font color="indigo"><b>Your fortune: Outlook good</b></font>|<font color="dodgerblue"><b>Your fortune: You will meet a dark handsome stranger</b></font>|<font color="darkorange"><b>Your fortune: Good Luck</b></font>|<font color="royalblue"><b>Your fortune: Better not tell you now</b></font>|<font color="deeppink"><b>Your fortune: Reply hazy; try again</b></font>|<font color="lime"><b>Your fortune: Very Bad Luck</b></font>|<font color="lime"><b>Your fortune: Good news will come to you by mail</b></font>|<font color="#BFC52F"><b>Your fortune: Average Luck</b></font>`
