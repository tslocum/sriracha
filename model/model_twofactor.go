package model

import (
	"html/template"

	. "codeberg.org/tslocum/sriracha/util"
)

// TwoFactor represents a two-factor authentication device.
type TwoFactor struct {
	ID         int
	Account    int
	Timestamp  int64
	LastActive int64
	Secret     string
	Name       string
}

func (t *TwoFactor) TimestampLabel() template.HTML {
	return FormatTimestamp(t.Timestamp)
}

func (t *TwoFactor) LastActiveLabel() template.HTML {
	return FormatTimestamp(t.LastActive)
}
