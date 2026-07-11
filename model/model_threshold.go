package model

import (
	"fmt"
	"html/template"
)

type ThresholdEvent int

const (
	EventPost   ThresholdEvent = 0
	EventThread ThresholdEvent = 1
	EventReport ThresholdEvent = 2
)

type Threshold struct {
	ID         int
	Everyone   bool
	Amount     int
	Event      ThresholdEvent
	Everywhere bool
	Duration   int
	Action     string
}

func (t *Threshold) Validate() error {
	if t.Amount <= 0 {
		return fmt.Errorf("invalid threshold amount")
	} else if t.Duration <= 0 {
		return fmt.Errorf("invalid threshold duration")
	}
	return nil
}

func (t *Threshold) Label() template.HTML {
	return ""
}
