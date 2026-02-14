package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/lrstanley/girc"
)

const (
	configSecure       = "secure"
	configSecureInfo   = "Enable SSL."
	configAddress      = "address"
	configAddressInfo  = "Server address. (Hostname:Port)\nBlank to disable plugin."
	configPassword     = "password"
	configPasswordInfo = "Server password."
	configNick         = "nick"
	configNickInfo     = "Nickname."
	configUser         = "user"
	configUserInfo     = "Username."
	configName         = "name"
	configNameInfo     = "Full name."
	configCreate       = "create"
	configCreateInfo   = "Channels to notify when a new post is created."
	configApprove      = "approve"
	configApproveInfo  = "Channels to notify when a post requires approval."
	configReport       = "report"
	configReportInfo   = "Channels to notify when a post is reported."
	configMod          = "mod"
	configModInfo      = "Channels to notify when a moderator takes action."
	configAdmin        = "admin"
	configAdminInfo    = "Channels to notify when an administrator takes action."
	configCommand      = "command"
	configCommandInfo  = "Sent before joining channels."
	configDebug        = "debug"
	configDebugInfo    = "Print connection info and events to console."

	nameShort = "sriracha"
	nameFull  = "Sriracha Imageboard and Forum"

	about = "Send server event notifications."

	portPlain  = "6667"
	portSecure = "6697"

	quitDelay      = 2 * time.Second
	commandDelay   = 2 * time.Second
	rebuildDelay   = 5 * time.Second
	reconnectDelay = 30 * time.Second
)

var help = template.HTML(`Channel format: <span style="border: 1px solid;padding: 2px;">#channel key</span>`)

type IRC struct {
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

	command string
	debug   bool

	client    *girc.Client
	rebuild   chan struct{}
	rebuildAt time.Time

	started   bool
	startedAt time.Time
}

// connect dials the server.
func (i *IRC) connect() {
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
		go i.scheduleRebuild(reconnectDelay)
		return
	}
}

// appendUnique parses channel names and keys from a slice of strings.
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

// joinChannels parses any channel keys from settings and then joins all channels.
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

func (i *IRC) onConnect(_ *girc.Client, _ girc.Event) {
	// Send user-defined commands.
	for _, cmd := range strings.Split(i.command, "\n") {
		cmd = strings.TrimPrefix(cmd, "/")
		if cmd == "" {
			continue
		}
		i.client.Cmd.SendRawNoSplit(cmd)
		time.Sleep(commandDelay)
	}

	// Join channels.
	i.joinChannels()
}

func (i *IRC) onPrivMsg(_ *girc.Client, e girc.Event) {
	msg := strings.ToLower(strings.TrimSpace(e.Last()))
	if msg != ".status" && msg != ".s" {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryUsage := FormatFileSize(int64(m.Sys))

	status := fmt.Sprintf("Goroutines: %d - Memory: %s - Uptime: %s", runtime.NumGoroutine(), memoryUsage, FormatDuration(time.Since(i.startedAt)))
	i.client.Cmd.Reply(e, girc.Fmt(status))
}

// rebuildClient disconnects the existing client (when connected) then builds
// and connects a new client.
func (i *IRC) rebuildClient() {
	// Disconnect existing client.
	if i.client != nil {
		i.client.Quit(nameFull)
		time.Sleep(quitDelay)
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
				port = portSecure
			} else {
				port = portPlain
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
	i.client.Handlers.Add(girc.CONNECTED, i.onConnect)
	i.client.Handlers.Add(girc.PRIVMSG, i.onPrivMsg)

	// Connect client.
	go i.connect()
}

// handleRebuild handles requests to rebuild the client.
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

		i.rebuildClient()
	}
}

// scheduleRebuild schedules the client to be rebuilt.
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

// postMessage formats a message referencing a post.
func (i *IRC) postMessage(post *Post, info string) string {
	return info + ": " + post.URL("")
}

func (i *IRC) About() string {
	if !i.started {
		i.rebuild = make(chan struct{})
		i.keys = make(map[string]string)
		go i.handleRebuild()
		i.started = true
		i.startedAt = time.Now()
	}
	return about
}

func (i *IRC) Help() template.HTML {
	return help
}

func (i *IRC) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:    sriracha.TypeBoolean,
			Name:    configSecure,
			Info:    configSecureInfo,
			Default: "1",
		}, {
			Type: sriracha.TypeString,
			Name: configAddress,
			Info: configAddressInfo,
		}, {
			Type:      sriracha.TypeString,
			Name:      configPassword,
			Info:      configPasswordInfo,
			Sensitive: true,
		}, {
			Type:    sriracha.TypeString,
			Name:    configNick,
			Info:    configNickInfo,
			Default: nameShort,
		}, {
			Type:    sriracha.TypeString,
			Name:    configUser,
			Info:    configUserInfo,
			Default: nameShort,
		}, {
			Type:    sriracha.TypeString,
			Name:    configName,
			Info:    configNameInfo,
			Default: nameFull,
		}, {
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configCreate,
			Info:     configCreateInfo,
		}, {
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configApprove,
			Info:     configApproveInfo,
		}, {
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configReport,
			Info:     configReportInfo,
		}, {
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configMod,
			Info:     configModInfo,
		}, {
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configAdmin,
			Info:     configAdminInfo,
		}, {
			Type:      sriracha.TypeString,
			Name:      configCommand,
			Info:      configCommandInfo,
			Sensitive: true,
		}, {
			Type: sriracha.TypeBoolean,
			Name: configDebug,
			Info: configDebugInfo,
		},
	}
}

func (i *IRC) Update(db sriracha.DB, key string) error {
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
	case configCommand:
		i.command = db.GetString(key)
	case configDebug:
		i.debug = db.GetBool(key)
	}
	go i.scheduleRebuild(0)
	return nil
}

func (i *IRC) Create(db sriracha.DB, post *Post) error {
	client := i.client
	if client == nil {
		return nil
	}
	if post.Moderated == ModeratedHidden {
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

func (i *IRC) Report(db sriracha.DB, post *Post) error {
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

func (i *IRC) Audit(db sriracha.DB, user string, action string, info string) error {
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

func Plugin() any {
	return &IRC{}
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &IRC{}
	_ sriracha.PluginWithHelp   = &IRC{}
	_ sriracha.PluginWithConfig = &IRC{}
	_ sriracha.PluginWithUpdate = &IRC{}
	_ sriracha.PluginWithCreate = &IRC{}
	_ sriracha.PluginWithReport = &IRC{}
	_ sriracha.PluginWithAudit  = &IRC{}
)
