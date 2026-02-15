package model

import (
	"fmt"
	"strings"

	. "codeberg.org/tslocum/sriracha/util"
)

var doctypePrefx = "<!DOCTYPE html>"

type Page struct {
	ID      int
	Path    string
	Content string
}

func (p *Page) Validate() error {
	p.Path = strings.TrimSpace(p.Path)
	if p.Path == "" || !FilePathPattern.MatchString(p.Path) || strings.HasPrefix(p.Path, ".") || strings.HasPrefix(p.Path, "/") || strings.HasPrefix(strings.ToLower(p.Path), "sriracha") {
		return fmt.Errorf("invalid page path: %s", p.Path)
	}
	return nil
}

func (p *Page) URL() string {
	url := "/" + p.Path
	if strings.HasSuffix(url, "/index") {
		return strings.TrimSuffix(url, "index")
	}
	return url + ".html"
}

func (p *Page) AddHeaderAndFooter() bool {
	return !strings.HasPrefix(p.Content, doctypePrefx)
}
