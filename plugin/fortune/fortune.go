package main

import (
	"codeberg.org/tslocum/sriracha"
)

type Fortune struct {
}

func (f *Fortune) About() string {
	return "Give your posters some good luck (or bad)."
}

func (f *Fortune) Post(db *sriracha.Database, post *sriracha.Post) error {
	return nil
}

func init() {
	sriracha.RegisterPlugin(&Fortune{})
}

var _ sriracha.Plugin = &Fortune{}
