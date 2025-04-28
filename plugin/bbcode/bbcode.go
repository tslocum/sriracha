package main

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha"
	"github.com/frustra/bbcode"
)

const (
	configBold          = "bold"
	configItalic        = "italic"
	configUnderline     = "underline"
	configStrikethrough = "strikethrough"
	configColor         = "color"
	configSize          = "size"
	configLink          = "link"
)

const enable = "1"

type BBCode struct {
	config  map[string]bool
	updated bool

	compiler bbcode.Compiler
}

func (f *BBCode) About() string {
	return "Format BBCode in post messages."
}

func (f *BBCode) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:        sriracha.TypeBoolean,
			Name:        configBold,
			Default:     enable,
			Description: "[b]Bold text[/b]",
		}, {
			Type:        sriracha.TypeBoolean,
			Name:        configItalic,
			Default:     enable,
			Description: "[i]Italic text[/i]",
		}, {
			Type:        sriracha.TypeBoolean,
			Name:        configUnderline,
			Default:     enable,
			Description: "[u]Underline text[/u]",
		}, {
			Type:        sriracha.TypeBoolean,
			Name:        configStrikethrough,
			Default:     enable,
			Description: "[s]Strikethrough text[/s]",
		}, {
			Type:        sriracha.TypeBoolean,
			Name:        configColor,
			Default:     enable,
			Description: "[color=blue]Blue text[/color]",
		}, {
			Type:        sriracha.TypeBoolean,
			Name:        configSize,
			Description: "[size=72]Size 72 text[/size]",
		}, {
			Type:        sriracha.TypeBoolean,
			Name:        configLink,
			Description: "[url=https://zoopz.org]Link text[/url]",
		},
	}
}

func (f *BBCode) Update(db *sriracha.Database, key string) error {
	f.config[key] = db.GetBool(key)
	f.updated = true
	return nil
}

func (f *BBCode) rebuildCompiler() {
	f.compiler = bbcode.NewCompiler(true, true)

	var disableTags = []string{
		"center",
		"code",
		"img",
		"quote",
	}
	for _, tagName := range disableTags {
		f.compiler.SetTag(tagName, nil)
	}

	var options = map[string]string{
		configBold:          "b",
		configItalic:        "i",
		configUnderline:     "u",
		configStrikethrough: "s",
		configColor:         "color",
	}
	for configName, tagName := range options {
		if !f.config[configName] {
			f.compiler.SetTag(tagName, nil)
		}
	}

	if !f.config[configSize] {
		f.compiler.SetTag("size", nil)
	} else {
		f.compiler.SetTag("size", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
			out, _ := bbcode.DefaultTagCompilers["size"](node)
			sizeClass := out.Attrs["class"]
			if strings.HasPrefix(sizeClass, "size") {
				size, err := strconv.Atoi(strings.TrimPrefix(sizeClass, "size"))
				if err == nil && size >= 1 {
					span := bbcode.NewHTMLTag("")
					span.Name = "span"
					span.Attrs["style"] = fmt.Sprintf("font-size: %dpt;", size)
					return span, true
				}
			}
			return bbcode.NewHTMLTag(""), true
		})
	}

	if !f.config[configLink] {
		f.compiler.SetTag("url", nil)
		return
	}
	validURL, err := regexp.Compile(`^([a-z][a-z0-9+\-.]*)://.*`)
	if err != nil {
		log.Fatal(err)
	}
	f.compiler.SetTag("url", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		out, appendExpr := bbcode.DefaultTagCompilers["url"](node)
		if strings.TrimSpace(out.Attrs["href"]) == "" {
			return nil, false
		} else if !validURL.MatchString(out.Attrs["href"]) || strings.HasPrefix(out.Attrs["href"], "javascript:") {
			text := bbcode.NewHTMLTag(html.EscapeString(out.Attrs["href"]))
			return text, false
		}
		return out, appendExpr
	})
}

func (f *BBCode) Post(db *sriracha.Database, post *sriracha.Post) error {
	if f.updated {
		f.rebuildCompiler()
		f.updated = false
	}

	post.Message = f.compiler.Compile(html.UnescapeString(post.Message))
	return nil
}

func init() {
	p := &BBCode{
		config:  make(map[string]bool),
		updated: true,
	}
	sriracha.RegisterPlugin(p)
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &BBCode{}
	_ sriracha.PluginWithConfig = &BBCode{}
	_ sriracha.PluginWithUpdate = &BBCode{}
	_ sriracha.PluginWithPost   = &BBCode{}
)
