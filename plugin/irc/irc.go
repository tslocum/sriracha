package main

import (
	"log"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha"
	"github.com/lrstanley/girc"
)

const (
	configSecure            = "secure"
	configSecureDescription = "Enable SSL."

	configAddress            = "address"
	configAddressDescription = "Server address. (Hostname:Port)\nBlank to disable plugin."

	configPassword            = "password"
	configPasswordDescription = "Server password."

	configNick            = "nick"
	configNickDescription = "Nickname."

	configUser            = "user"
	configUserDescription = "Username."

	configName            = "name"
	configNameDescription = "Full name."

	configCreate            = "create"
	configCreateDescription = "Channels to notify when a new post is created."

	configApprove            = "approve"
	configApproveDescription = "Channels to notify when a post requires approval."

	configReport            = "report"
	configReportDescription = "Channels to notify when a post is reported."

	configMod            = "mod"
	configModDescription = "Channels to notify when a moderator takes action."

	configAdmin            = "admin"
	configAdminDescription = "Channels to notify when an administrator takes action."

	configDebug            = "debug"
	configDebugDescription = "Print connection info and events to console."
)

const (
	rebuildDelay = 5 * time.Second
	nameShort    = "sriracha"
	nameFull     = "Sriracha Imageboard and Forum"
)

type IRC struct {
	client *girc.Client

	rebuild   chan struct{}
	rebuildAt time.Time

	secure   bool
	address  string
	password string
	nick     string
	user     string
	name     string

	create  []string
	approve []string
	report  []string
	mod     []string
	admin   []string
	keys    map[string]string

	debug bool

	started bool
}

func (i *IRC) connectBot() {
	if i.debug {
		var extra string
		if !i.secure {
			extra = " without"
		}
		log.Printf("Connecting to %s%s using SSL...", i.address, extra)
	}
	err := i.client.Connect()
	if err != nil {
		log.Printf("Warning: IRC plugin failed to connect to server %s: %s", i.address, err)
		go i.scheduleRebuild(30 * time.Second)
		return
	}
}

func (i *IRC) appendUnique(channels []string, chs []string) []string {
	for _, ch := range chs {
		if ch == "" {
			continue
		}
		split := strings.SplitN(ch, " ", 2)
		ch = split[0]
		if slices.Contains(channels, ch) {
			continue
		}

		channels = append(channels, ch)

		if len(split) == 2 && split[1] != "" {
			i.keys[ch] = split[1]
		}
	}
	return channels
}

func (i *IRC) joinChannels() {
	clear(i.keys)

	channels := i.appendUnique(nil, i.create)
	channels = i.appendUnique(channels, i.approve)
	channels = i.appendUnique(channels, i.report)
	channels = i.appendUnique(channels, i.mod)
	channels = i.appendUnique(channels, i.admin)
	for _, ch := range channels {
		key := i.keys[ch]
		if key == "" {
			i.client.Cmd.Join(ch)
		} else {
			i.client.Cmd.JoinKey(ch, key)
		}
	}
}

func (i *IRC) rebuildBot() {
	// Disconnect existing client.
	if i.client != nil {
		i.client.Quit(nameFull)
		time.Sleep(2 * time.Second)
	}

	if i.address == "" {
		return
	}

	// Parse server address.
	hostname, port, err := net.SplitHostPort(i.address)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			hostname = i.address
			if i.secure {
				port = "6697"
			} else {
				port = "6667"
			}
		} else {
			log.Printf("Warning: IRC plugin failed to parse server address %s: %s", i.address, err)
			return
		}
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		log.Printf("Warning: IRC plugin failed to parse server port %s: %s", i.address, err)
		return
	}

	// Initialize client configuration.
	config := girc.Config{
		SSL:        i.secure,
		Server:     hostname,
		Port:       portNum,
		ServerPass: i.password,
		Nick:       i.nick,
		User:       i.user,
		Name:       i.name,
		Version:    nameFull,
	}
	if i.debug {
		config.Debug = os.Stderr
	}

	// Create client.
	i.client = girc.New(config)

	// Set handlers.
	i.client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		i.joinChannels()
	})

	// Connect client.
	go i.connectBot()
}

func (i *IRC) handleRebuild() {
	for {
		<-i.rebuild

	WAITREBUILD:
		for {
			select {
			case <-i.rebuild:
			default:
				time.Sleep(100 * time.Millisecond)
				if time.Now().After(i.rebuildAt) {
					break WAITREBUILD
				}
			}
		}

		i.rebuildBot()
	}
}

func (i *IRC) scheduleRebuild(delay time.Duration) {
	if delay == 0 {
		delay = rebuildDelay
	}
	rebuildAt := time.Now().Add(delay)
	if i.rebuildAt.After(rebuildAt) {
		return
	}
	i.rebuildAt = rebuildAt
	i.rebuild <- struct{}{}
}

func (i *IRC) About() string {
	if !i.started {
		i.rebuild = make(chan struct{})
		i.keys = make(map[string]string)
		go i.handleRebuild()
		i.started = true
	}
	return "Send server event notifications."
}

func (i *IRC) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:        sriracha.TypeBoolean,
			Name:        configSecure,
			Description: configSecureDescription,
			Default:     "1",
		},
		{
			Type:        sriracha.TypeString,
			Name:        configAddress,
			Description: configAddressDescription,
		},
		{
			Type:        sriracha.TypeString,
			Name:        configPassword,
			Description: configPasswordDescription,
		},
		{
			Type:        sriracha.TypeString,
			Name:        configNick,
			Description: configNickDescription,
			Default:     nameShort,
		},
		{
			Type:        sriracha.TypeString,
			Name:        configUser,
			Description: configUserDescription,
			Default:     nameShort,
		},
		{
			Type:        sriracha.TypeString,
			Name:        configName,
			Description: configNameDescription,
			Default:     nameFull,
		},
		{
			Type:        sriracha.TypeString,
			Multiple:    true,
			Name:        configCreate,
			Description: configCreateDescription,
		},
		{
			Type:        sriracha.TypeString,
			Multiple:    true,
			Name:        configApprove,
			Description: configApproveDescription,
		},
		{
			Type:        sriracha.TypeString,
			Multiple:    true,
			Name:        configReport,
			Description: configReportDescription,
		},
		{
			Type:        sriracha.TypeString,
			Multiple:    true,
			Name:        configMod,
			Description: configModDescription,
		},
		{
			Type:        sriracha.TypeString,
			Multiple:    true,
			Name:        configAdmin,
			Description: configAdminDescription,
		},
		{
			Type:        sriracha.TypeBoolean,
			Name:        configDebug,
			Description: configDebugDescription,
		},
	}
}

func (i *IRC) Update(db *sriracha.Database, key string) error {
	switch key {
	case configSecure:
		i.secure = db.GetBool(key)
	case configAddress:
		i.address = db.GetString(key)
	case configPassword:
		i.password = db.GetString(key)
	case configNick:
		i.nick = db.GetString(key)
		if i.nick == "" {
			i.nick = nameShort
		}
	case configUser:
		i.user = db.GetString(key)
		if i.user == "" {
			i.user = nameShort
		}
	case configName:
		i.name = db.GetString(key)
		if i.name == "" {
			i.name = nameFull
		}
	case configCreate:
		i.create = db.GetMultiString(key)
	case configApprove:
		i.approve = db.GetMultiString(key)
	case configReport:
		i.report = db.GetMultiString(key)
	case configMod:
		i.mod = db.GetMultiString(key)
	case configAdmin:
		i.admin = db.GetMultiString(key)
	case configDebug:
		i.debug = db.GetBool(key)
	}
	go i.scheduleRebuild(0)
	return nil
}

func (i *IRC) postMessage(post *sriracha.Post, info string) string {
	message := info + ": " + post.URL()
	if post.Subject != "" {
		message += " " + post.Subject
	}
	return message
}

func (i *IRC) Create(db *sriracha.Database, post *sriracha.Post) error {
	client := i.client
	if client == nil {
		return nil
	}
	if post.Moderated == sriracha.ModeratedHidden {
		for _, ch := range i.approve {
			if ch == "" {
				continue
			}
			client.Cmd.Message(ch, i.postMessage(post, "Post pending"))
		}
	} else {
		for _, ch := range i.create {
			if ch == "" {
				continue
			}
			client.Cmd.Message(ch, i.postMessage(post, "Post created"))
		}
	}
	return nil
}

func (i *IRC) Report(db *sriracha.Database, post *sriracha.Post) error {
	client := i.client
	if client == nil {
		return nil
	}
	for _, ch := range i.report {
		if ch == "" {
			continue
		}
		client.Cmd.Message(ch, i.postMessage(post, "Post reported"))
	}
	return nil
}

func (i *IRC) Audit(db *sriracha.Database, user string, action string, info string) error {
	client := i.client
	if client == nil {
		return nil
	}
	message := strings.Title(user) + ": " + action
	var channels []string
	if user == "system" || user == "admin" {
		channels = i.admin
	} else {
		channels = i.mod
	}
	for _, ch := range channels {
		if ch == "" {
			continue
		}
		client.Cmd.Message(ch, message)
	}
	return nil
}

func init() {
	sriracha.RegisterPlugin(&IRC{})
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &IRC{}
	_ sriracha.PluginWithConfig = &IRC{}
	_ sriracha.PluginWithUpdate = &IRC{}
	_ sriracha.PluginWithCreate = &IRC{}
	_ sriracha.PluginWithReport = &IRC{}
	_ sriracha.PluginWithAudit  = &IRC{}
)
