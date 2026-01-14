package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha"
	"github.com/lrstanley/girc"
)

// Enable debug to print IRC connection information and events.
const debug = true

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

	started bool
}

func (i *IRC) connectBot() {
	err := i.client.Connect()
	if err != nil {
		log.Printf("Warning: IRC plugin failed to connect to server %s: %s", i.address, err)
		go i.scheduleRebuild(30 * time.Second)
		return
	}
}

func (i *IRC) rebuildBot() {
	// Disconnect existing client.
	if i.client != nil {
		i.client.Quit(nameFull)
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

	// Create new client.
	i.client = girc.New(girc.Config{
		SSL:        i.secure,
		Server:     hostname,
		Port:       portNum,
		ServerPass: i.password,
		Nick:       i.nick,
		User:       i.user,
		Name:       i.name,
		Version:    nameFull,
	})

	if debug {
		i.client.Config.Debug = os.Stdout
	}

	// Set handlers.
	i.client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		// Join channels.
		// TODO Key support
		for _, ch := range i.create {
			if ch == "" {
				continue
			}
			i.client.Cmd.Join(ch)
		}
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
)
