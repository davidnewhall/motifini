package cli

import (
	"fmt"
	"log"
)

/* Later we can add more logic around log routes. */

// Log satisfies external logger interfaces.
type Log struct {
	Muted bool
	Affix string
}

// Print prints a log message without formatting.
func (l *Log) Print(v ...interface{}) {
	if l.Muted {
		return
	}
	_ = log.Output(2, l.Affix+fmt.Sprint(v...))
}

// Printf prints a log message with formatting.
func (l *Log) Printf(msg string, v ...interface{}) {
	if l.Muted {
		return
	}
	_ = log.Output(2, l.Affix+fmt.Sprintf(msg, v...))
}

// Println prints a log message with spaces between each element.
func (l *Log) Println(v ...interface{}) {
	if l.Muted {
		return
	}
	_ = log.Output(2, l.Affix+fmt.Sprintln(v...))
}
