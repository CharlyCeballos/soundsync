package i18n

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const goLit = `"(?:[^"\\\n]|\\.)*"`

// Sources of messages that reach i18n.T indirectly, besides i18n.T("...").
var msgPatterns = []*regexp.Regexp{
	regexp.MustCompile(`i18n\.(?:T|Error)\((` + goLit + `)\)`),
	regexp.MustCompile(`command\(a, ` + goLit + `, [^,]+,\s*(` + goLit + `)`), // Short, translated in command()
	regexp.MustCompile(`confirm\(out, (` + goLit + `)\)`),                     // question, translated in confirm()
}

var privacyNoticeRE = regexp.MustCompile("(?s)const privacyNotice = `(.*?)`")

// identical lists messages whose Spanish is the same text.
var identical = map[string]bool{"OK": true, "Error: %v\n": true, "  %d. %q (%s, %d videos) %s\n": true, "%q (%d videos)": true}

func cliMessages(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "cmd", "soundsync", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no CLI sources found: %v", err)
	}
	var msgs []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, re := range msgPatterns {
			for _, m := range re.FindAllStringSubmatch(string(src), -1) {
				s, err := strconv.Unquote(m[1])
				if err != nil {
					t.Fatalf("%s: %v", f, err)
				}
				msgs = append(msgs, s)
			}
		}
		if m := privacyNoticeRE.FindStringSubmatch(string(src)); m != nil {
			msgs = append(msgs, m[1])
		}
	}
	// match.Reason labels shown in the WHY column.
	return append(msgs, "first", "playlist")
}

var verbRE = regexp.MustCompile(`%(?:\[\d+\])?[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)

func verbs(s string) []string {
	var out []string
	for _, v := range verbRE.FindAllString(s, -1) {
		out = append(out, v[len(v)-1:])
	}
	return out
}

func TestEveryCLIMessageHasSpanish(t *testing.T) {
	letters := regexp.MustCompile(`[A-Za-z]{2}`)
	msgs := cliMessages(t)
	if len(msgs) < 140 {
		t.Fatalf("found only %d messages: the source patterns no longer match the CLI code", len(msgs))
	}
	for _, msg := range msgs {
		if !letters.MatchString(msg) {
			continue // pure format strings such as "%q: %w"
		}
		es, ok := es[msg]
		if !ok {
			t.Errorf("no Spanish for %q", msg)
			continue
		}
		if es == msg && !identical[msg] {
			t.Errorf("Spanish equals English for %q; add it to identical if intended", msg)
		}
		if strings.Join(verbs(es), "") != strings.Join(verbs(msg), "") && !strings.Contains(es, "%[") {
			t.Errorf("fmt verbs differ:\n  en %q\n  es %q", msg, es)
		}
	}
}

func TestFromLocale(t *testing.T) {
	for in, want := range map[string]string{"es_MX.UTF-8": ES, "es-MX": ES, "en-US": EN, "EN_gb": EN, "fr-FR": "", "": "", "C": ""} {
		if got := fromLocale(in); got != want {
			t.Errorf("fromLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetAndTranslatedErrors(t *testing.T) {
	t.Cleanup(func() { current = EN })
	if err := Set("fr"); err == nil {
		t.Error("unsupported language must fail")
	}
	errDeclined := Error("unclosed quote")
	wrapped := errors.Join(errors.New("context"), errDeclined)
	if !errors.Is(wrapped, errDeclined) || errDeclined.Error() != "unclosed quote" {
		t.Fatal("Error must stay comparable and print English by default")
	}
	if err := Set(ES); err != nil {
		t.Fatal(err)
	}
	if errDeclined.Error() != "comilla sin cerrar" || T("never translated") != "never translated" {
		t.Errorf("Spanish lookups broken: %q", errDeclined.Error())
	}
}
