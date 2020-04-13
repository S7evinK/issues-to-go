package cmd

import (
	"fmt"
	"time"
)

// https://blog.gopheracademy.com/advent-2019/cmdline/

var spinChars = `|/-\`

// Spinner defines a terminal spinner
type Spinner struct {
	i       int
	chClose chan bool
}

// NewSpinner creates a new spinner
func NewSpinner(closeChannel chan bool) *Spinner {
	return &Spinner{chClose: closeChannel}
}

// Tick executes every x milliseconds
func (s *Spinner) Tick() {
	fmt.Printf(" %c \r", spinChars[s.i])
	s.i = (s.i + 1) % len(spinChars)
}

// Run starts the spinner
func (s *Spinner) Run() {
	t := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-t.C:
			s.Tick()
		case <-s.chClose:
			return
		}
	}
}
