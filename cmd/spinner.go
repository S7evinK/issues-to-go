package cmd

import (
	"fmt"
	"time"
)

// https://blog.gopheracademy.com/advent-2019/cmdline/

var spinChars = `|/-\`

type Spinner struct {
	i       int
	chClose chan bool
}

func NewSpinner(closeChannel chan bool) *Spinner {
	return &Spinner{chClose: closeChannel}
}

func (s *Spinner) Tick() {
	fmt.Printf(" %c \r", spinChars[s.i])
	s.i = (s.i + 1) % len(spinChars)
}

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
