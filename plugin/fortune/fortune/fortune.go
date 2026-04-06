package fortune

import (
	"math/rand"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
)

const (
	configTriggers = "triggers"
	configFortunes = "fortunes"

	defaultTrigger  = "#fortune"
	defaultFortunes = `<span style="color: #B604A2;"><b>Your fortune: Godly Luck</b></span>|||<span style="color: indigo;"><b>Your fortune: Outlook good</b></span>|||<span style="color: dodgerblue;"><b>Your fortune: You will meet a dark handsome stranger</b></span>|||<span style="color: darkorange;"><b>Your fortune: Good Luck</b></span>|||<span style="color: royalblue;"><b>Your fortune: Better not tell you now</b></span>|||<span style="color: deeppink;"><b>Your fortune: Reply hazy; try again</b></span>|||<span style="color: lime;"><b>Your fortune: Very Bad Luck</b></span>|||<span style="color: lime;"><b>Your fortune: Good news will come to you by mail</b></span>|||<span style="color: #BFC52F;"><b>Your fortune: Average Luck</b></span>`
)

type Fortune struct{}

func NewFortune() *Fortune {
	return &Fortune{}
}

func (f *Fortune) About() string {
	return "Give your visitors some good luck (or bad)."
}

func (f *Fortune) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configTriggers,
			Default:  defaultTrigger,
			Info:     "The text users may input in the name or email field to receive a fortune.",
		}, {
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configFortunes,
			Default:  defaultFortunes,
			Info:     "The fortunes users may receive.",
		},
	}
}

func (f *Fortune) Post(db sriracha.DB, post *Post) error {
	var showFortune bool
	for _, trigger := range db.GetMultiString(configTriggers) {
		if strings.EqualFold(post.Name, trigger) {
			post.Name = ""
			showFortune = true
			break
		}
		if strings.EqualFold(post.Email, trigger) {
			post.Email = ""
			showFortune = true
			break
		}
	}
	if !showFortune {
		return nil
	}

	fortunes := db.GetMultiString(configFortunes)
	if len(fortunes) == 0 {
		return nil
	}

	fortune := fortunes[rand.Intn(len(fortunes))]
	if len(strings.TrimSpace(post.Message)) == 0 {
		post.Message = fortune
	} else {
		post.Message = fortune + "\n\n" + post.Message
	}
	return nil
}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &Fortune{}
	_ sriracha.PluginWithConfig = &Fortune{}
	_ sriracha.PluginWithPost   = &Fortune{}
)
