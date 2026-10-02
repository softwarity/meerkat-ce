// Package logging decides what the gateway's own output looks like (OBS-03).
//
// Until this existed there was no handler at all, so Go's default stood: text
// on standard error, at Info, with no way to ask for more. Which meant the one
// thing an operator wants at two in the morning - turn the level up, look, turn
// it back down - was a redeploy.
//
// TWO STREAMS OF VERY DIFFERENT SIZE come out of here, and keeping them
// straight is the whole design. The operational log is a few thousand lines a
// day: startup, reloads, upstreams going down. The ACCESS log is one line per
// request, which at four hundred a second is thirty-five million lines a day.
//
// They share one stream anyway, because that is what a container gives you,
// and they stay separable because every line is a typed object: an access line
// carries `"type":"access"`, and a collector routes on it in three lines of
// configuration, to two destinations with two retentions. Two streams of our
// own - stdout against stderr, the nginx way - would be a separation most
// collectors silently merge back, which is worse than not separating at all.
//
// AND THE ACCESS LOG SHIPS OFF, because of that same asymmetry: an operator
// running `docker logs` to find out why the gateway will not start must not
// have to scroll past forty thousand requests. Whoever turns it on knows why
// their stream got busy.
//
// Nothing is written to a file, and that is a decision. A file needs a mounted
// volume, a rotation, and an answer to "the disk is full" that must never be
// "stop serving traffic" - which is reimplementing logrotate inside a gateway.
// The runtime already rotates what a container writes to its standard output,
// and the operations team has already configured it.
package logging

import (
	"log/slog"
	"strings"
)

// Format is how a line is written.
type Format string

const (
	// FormatJSON is what a collector reads. The default wherever this looks
	// like a deployment rather than somebody's laptop.
	FormatJSON Format = "json"
	// FormatText is what a person reads, and what `make dev` gets.
	FormatText Format = "text"
)

// level is the live level, swapped without restarting anything: slog reads it
// through the pointer on every record, which is exactly what "turn it up for
// five minutes" needs.
var level = new(slog.LevelVar)

// Options is what startup settled on.
type Options struct {
	// Format, empty meaning "decide from Production".
	Format Format
	// Level, empty meaning Info.
	Level string
	// Production picks the default format: a deployment wants JSON, a laptop
	// wants to be readable.
	Production bool
}

// Setup installs the handler and returns the format actually chosen, for the
// startup line to report it - a silent decision about log format is the kind
// of thing somebody discovers three weeks later in a broken pipeline.
func Setup(o Options) Format {
	level.Set(ParseLevel(o.Level))
	startup = level.Level()

	format := o.Format
	if format != FormatJSON && format != FormatText {
		format = FormatText
		if o.Production {
			format = FormatJSON
		}
	}

	// Standard ERROR for what the gateway says about itself, which leaves
	// standard output free to mean one thing only: the access log. A person
	// tailing one is not drowned by the other even before a collector has
	// looked at the type field.
	base.Store(format)
	install()
	return format
}

// SetLevel changes what is written from now on. Live, and that is the point.
func SetLevel(name string) slog.Level {
	l := ParseLevel(name)
	level.Set(l)
	return l
}

// Level is what is being written right now.
func Level() slog.Level { return level.Level() }

// ParseLevel reads the four names an operator types. Anything else is Info
// rather than a refusal: a typo in a log level must not stop a gateway from
// starting, and Info is the level that tells them about the typo.
func ParseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LevelName writes a level back the way it was typed, for an API to answer
// with and a console to show.
func LevelName(l slog.Level) string {
	switch {
	case l <= slog.LevelDebug:
		return "debug"
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	default:
		return "info"
	}
}
