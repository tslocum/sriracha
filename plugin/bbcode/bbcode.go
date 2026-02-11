package main

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/tslocum/bbcode"
	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	"github.com/alecthomas/chroma/v2"
	htmlformatter "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

const (
	configBold          = "bold"
	configItalic        = "italic"
	configUnderline     = "underline"
	configStrikethrough = "strikethrough"
	configSpoiler       = "spoiler"
	configColor         = "color"
	configSize          = "size"
	configURL           = "URL"
	configPre           = "pre"
	configCode          = "code"
	configSJIS          = "SJIS"

	enable = "1"

	tabWidth  = 4
	codeStyle = "xcode-dark"

	maxSize = 100
)

var codePattern = regexp.MustCompile(`(?ms)\[code=?([^\]]+)?\].*?\[\/code\]`)

type BBCode struct {
	config  map[string]bool
	updated bool

	compiler bbcode.Compiler

	formatter *htmlformatter.Formatter
	style     *chroma.Style
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
			Name: configURL,
			Info: "[url=https://zoopz.org]Link text[/url]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configPre,
			Info: "[pre]Preformatted text[/pre]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configCode,
			Info: "[code=go]msg := \"Hello, world!\"[/code]",
		}, {
			Type: sriracha.TypeBoolean,
			Name: configSJIS,
			Info: "[sjis]Shift JIS text art[/sjis]",
		},
	}
}

func (f *BBCode) Update(db sriracha.DB, key string) error {
	f.config[key] = db.GetBool(key)
	f.updated = true
	return nil
}

func (f *BBCode) rebuildCompiler() {
	f.compiler = bbcode.NewCompiler(true, true)

	formatterOptions := []htmlformatter.Option{
		htmlformatter.Standalone(false),
		htmlformatter.TabWidth(tabWidth),
		htmlformatter.WrapLongLines(false),
		htmlformatter.WithLineNumbers(true),
		htmlformatter.LineNumbersInTable(true),
	}
	f.formatter = htmlformatter.New(formatterOptions...)

	f.style = styles.Get(codeStyle)
	if f.style == nil {
		f.style = styles.Fallback
	}

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
					if size > maxSize {
						size = maxSize
					}
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
			valueStr = strings.ReplaceAll(valueStr, "\n", newLineSentinel)
			node.Value = valueStr
		}
	}
	preFunc := func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		out := bbcode.NewHTMLTag("")
		out.Name = "span"
		out.Attrs["class"] = "code"
		for _, child := range node.Children {
			replaceNewLines(child)
			out.AppendChild(bbcode.CompileRaw(child))
		}
		return out, false
	}
	if !f.config[configPre] {
		f.compiler.SetTag("pre", nil)
	} else {
		f.compiler.SetTag("pre", preFunc)
	}

	out := &bytes.Buffer{}
	bracketSentinel := "\x1e" // Record separator
	codeFunc := func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		var lexer chroma.Lexer
		var foundLexer bool
		if openingTag, ok := node.Token.Value.(bbcode.BBOpeningTag); ok {
			language := strings.ToLower(strings.TrimSpace(openingTag.Value))
			if language != "" {
				lexer = lexers.Get(language)
				if lexer != nil {
					lexer = chroma.Coalesce(lexer)
					foundLexer = true
				}
			}
		}
		tag, appendExpr := preFunc(node)
		tag.Name = "div"
		tag.Attrs["class"] = "codeblock"
		for i, child := range tag.Children {
			strValue := strings.ReplaceAll(child.Value, newLineSentinel, "\n")
			strValue = strings.ReplaceAll(strValue, bracketSentinel, "[")
			strValue = strings.TrimSpace(strValue)
			if !foundLexer {
				lexer = lexers.Analyse(strValue)
				if lexer == nil {
					lexer = lexers.Fallback
				}
				lexer = chroma.Coalesce(lexer)
				foundLexer = true
			}
			iterator, err := lexer.Tokenise(nil, strValue)
			if err != nil {
				continue
			}
			out.Reset()
			err = f.formatter.Format(out, f.style, iterator)
			if err != nil {
				continue
			}
			tag.Children[i] = bbcode.NewHTMLTag("")
			tag.Children[i].Value = strings.ReplaceAll(out.String(), "\n", newLineSentinel)
			tag.Children[i].Raw = true
		}
		return tag, appendExpr
	}
	if !f.config[configCode] {
		f.compiler.SetTag("code", nil)
	} else {
		f.compiler.SetTag("code", codeFunc)
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

	if !f.config[configURL] {
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

func (f *BBCode) Post(db sriracha.DB, post *Post) error {
	if f.updated {
		f.rebuildCompiler()
		f.updated = false
	}

	bracketSentinel := byte('\x1e') // Record separator
	post.Message = codePattern.ReplaceAllStringFunc(post.Message, func(s string) string {
		firstBracket := strings.IndexByte(s, '[')
		lastBracket := strings.LastIndexByte(s, '[')
		if firstBracket == -1 || lastBracket == -1 {
			return s
		}
		var out []byte
		for i, c := range s {
			if c == '[' && i != firstBracket && i != lastBracket {
				out = append(out, bracketSentinel)
			} else {
				out = append(out, []byte(string(c))...)
			}
		}
		return string(out)
	})
	post.Message = strings.ReplaceAll(post.Message, "[/code]\n", "[/code]")

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
