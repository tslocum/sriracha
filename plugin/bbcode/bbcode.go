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
	configSpoiler       = "spoiler"
	configColor         = "color"
	configSize          = "size"
	configLink          = "link"
	configCode          = "code"
	configSJIS          = "SJIS"
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
			Type:    sriracha.TypeBoolean,
			Name:    configBold,
			Default: enable,
			Info:    "[b]Bold text[/b]",
		}, {
			Type:    sriracha.TypeBoolean,
			Name:    configItalic,
			Default: enable,
			Info:    "[i]Italic text[/i]",
		}, {
			Type:    sriracha.TypeBoolean,
			Name:    configUnderline,
			Default: enable,
			Info:    "[u]Underline text[/u]",
		}, {
			Type:    sriracha.TypeBoolean,
			Name:    configStrikethrough,
			Default: enable,
			Info:    "[s]Strikethrough text[/s]",
		}, {
			Type:    sriracha.TypeBoolean,
			Name:    configSpoiler,
			Default: enable,
			Info:    "[spoiler]Spoiler text[/spoiler]",
		}, {
			Type:    sriracha.TypeBoolean,
			Name:    configColor,
			Default: enable,
			Info:    "[color=blue]Blue text[/color]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configSize,
			Info: "[size=72]Size 72 text[/size]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configLink,
			Info: "[url=https://zoopz.org]Link text[/url]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configCode,
			Info: "[code]$str = \"Hello, world!\";[/code]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configSJIS,
			Info: "[sjis]Shift JIS text art[/sjis]",
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
		configSpoiler:       "spoiler",
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

	if !f.config[configSpoiler] {
		f.compiler.SetTag("spoiler", nil)
	} else {
		f.compiler.SetTag("spoiler", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
			span := bbcode.NewHTMLTag("")
			span.Name = "span"
			span.Attrs["class"] = "spoiler"
			return span, true
		})
	}

	newLineSentinel := "\x85" // Next line (NEL) character
	var replaceNewLines func(*bbcode.BBCodeNode)
	replaceNewLines = func(node *bbcode.BBCodeNode) {
		for _, child := range node.Children {
			replaceNewLines(child)
		}
		valueStr, ok := node.Value.(string)
		if ok {
			node.Value = strings.ReplaceAll(valueStr, "\n", newLineSentinel)
		}
	}
	codeFunc := func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		out := bbcode.NewHTMLTag("")
		out.Name = "span"
		out.Attrs["class"] = "code"
		for _, child := range node.Children {
			replaceNewLines(child)
			out.AppendChild(bbcode.CompileRaw(child))
		}
		return out, false
	}
	for _, tag := range []string{"code", "pre"} {
		if !f.config[configCode] {
			f.compiler.SetTag(tag, nil)
		} else {
			f.compiler.SetTag(tag, codeFunc)
		}
	}

	if !f.config[configSJIS] {
		f.compiler.SetTag("sjis", nil)
	} else {
		f.compiler.SetTag("sjis", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
			out := bbcode.NewHTMLTag("")
			out.Name = "span"
			out.Attrs["class"] = "sjis"
			for _, child := range node.Children {
				replaceNewLines(child)
				out.AppendChild(bbcode.CompileRaw(child))
			}
			return out, false
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

func Plugin() any {
	return &BBCode{
		config:  make(map[string]bool),
		updated: true,
	}
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &BBCode{}
	_ sriracha.PluginWithConfig = &BBCode{}
	_ sriracha.PluginWithUpdate = &BBCode{}
	_ sriracha.PluginWithPost   = &BBCode{}
)
