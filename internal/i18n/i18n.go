package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Supported languages (planV1 §4: es-MX and en).
const (
	EN = "en"
	ES = "es"
)

// current is process-wide (one user). It is atomic because the GUI switches
// language while background work is translating messages.
var current atomic.Value // string; unset means EN

// Set selects the language for every later message.
func Set(lang string) error {
	switch lang {
	case EN, ES:
		current.Store(lang)
		return nil
	}
	return fmt.Errorf("unsupported language %q (want es or en)", lang)
}

// Current returns the language messages are translated to.
func Current() string {
	if lang, ok := current.Load().(string); ok {
		return lang
	}
	return EN
}

// T translates an English message; the English text is the key (gettext
// style). A message without a translation stays in English.
func T(msg string) string {
	if Current() == ES {
		if t, ok := es[msg]; ok {
			return t
		}
	}
	return msg
}

// Error is an error whose message is translated when printed, so
// package-level sentinel errors follow the language chosen at run time.
// Being a comparable value, it still works with errors.Is.
type Error string

func (e Error) Error() string { return T(string(e)) }

// Default is the system's language when it is Spanish or English, else
// Spanish (planV1 §13).
func Default() string {
	if lang := fromLocale(systemLocale()); lang != "" {
		return lang
	}
	return ES
}

// systemLocale reads the usual POSIX variables first; on Windows, where
// they are normally unset, osLocale asks the OS.
func systemLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return osLocale()
}

// fromLocale maps "es_MX.UTF-8", "es-MX" or "en-US" to es or en.
func fromLocale(locale string) string {
	l := strings.ToLower(locale)
	switch {
	case strings.HasPrefix(l, "es"):
		return ES
	case strings.HasPrefix(l, "en"):
		return EN
	}
	return ""
}
