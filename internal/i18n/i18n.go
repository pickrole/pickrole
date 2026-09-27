// Package i18n translates the messages the backend shows to the user.
//
// The UI picks the language (from the preference or the system) and sets it
// with SetLanguage. An *Error looks its message up when it is displayed;
// wrapping it with fmt.Errorf fixes the text at that moment, which is fine
// for errors created while handling a UI call. English is the fallback for
// missing translations.
package i18n

import (
	"fmt"
	"sync/atomic"
)

// Lang is a supported language.
type Lang string

// Supported languages.
const (
	English    Lang = "en"
	Portuguese Lang = "pt-BR"
)

var current atomic.Value // Lang

func init() { current.Store(English) }

// SetLanguage sets the language of every message from now on. Unknown
// values fall back to English.
func SetLanguage(l Lang) {
	if _, ok := catalog[l]; !ok {
		l = English
	}
	current.Store(l)
}

// Language returns the current language.
func Language() Lang { return current.Load().(Lang) }

// T returns the message for key in the current language, formatted with
// args like fmt.Sprintf.
func T(key string, args ...any) string {
	msg, ok := catalog[Language()][key]
	if !ok {
		msg, ok = catalog[English][key]
	}
	if !ok {
		msg = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// Error is a translatable error. Its text is looked up on every call to
// Error, and it can wrap a cause.
type Error struct {
	key   string
	args  []any
	cause error
}

// New returns an error with the message for key. Without args it works as a
// sentinel for errors.Is.
func New(key string, args ...any) *Error { return &Error{key: key, args: args} }

// Wrap returns an error "message: cause" that errors.Is/As see through.
func Wrap(key string, cause error, args ...any) error {
	return &Error{key: key, args: args, cause: cause}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return T(e.key, e.args...) + ": " + e.cause.Error()
	}
	return T(e.key, e.args...)
}

func (e *Error) Unwrap() error { return e.cause }

// Key identifies the message, for tests and callers that branch on it.
func (e *Error) Key() string { return e.key }
