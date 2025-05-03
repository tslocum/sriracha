package main

import (
	"fmt"

	"codeberg.org/tslocum/sriracha"
)

const (
	configMessage = "message"
	configBoards  = "boards"
)

type Robot9000 struct {
	message error
	boards  []int
}

func (r *Robot9000) About() string {
	return "Require post messages to be unique."
}

func (r *Robot9000) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:        sriracha.TypeString,
			Name:        configMessage,
			Description: "Error message shown when a post is denied.",
			Default:     "This board only allows unique posts to be created. Please enter a unique message and try again.",
		},
		{
			Type:        sriracha.TypeBoard,
			Multiple:    true,
			Name:        configBoards,
			Description: "Only allow unique posts to be created in the selected boards.",
		},
	}
}

func (r *Robot9000) Update(db *sriracha.Database, key string) error {
	switch key {
	case configMessage:
		r.message = fmt.Errorf("%s", db.GetString(configMessage))
	case configBoards:
		r.boards = db.GetMultiInt(configBoards)
	}
	return nil
}

func (r *Robot9000) Insert(db *sriracha.Database, post *sriracha.Post) error {
	if post.Message == "" {
		return nil
	}

	var found bool
	for _, boardID := range r.boards {
		if boardID == post.Board.ID {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	existing := db.PostByField(post.Board, "message", post.Message)
	if existing != nil {
		return r.message
	}
	return nil
}

func init() {
	sriracha.RegisterPlugin(&Robot9000{})
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &Robot9000{}
	_ sriracha.PluginWithConfig = &Robot9000{}
	_ sriracha.PluginWithUpdate = &Robot9000{}
	_ sriracha.PluginWithInsert = &Robot9000{}
)
