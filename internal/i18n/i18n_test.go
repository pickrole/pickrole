package i18n

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Every language has every English key, and the same format verbs.
func TestCatalogIsComplete(t *testing.T) {
	for lang, msgs := range catalog {
		for key, en := range catalog[English] {
			msg, ok := msgs[key]
			if !ok {
				t.Errorf("%s: missing %q", lang, key)
				continue
			}
			if verbs(msg) != verbs(en) {
				t.Errorf("%s: %q has verbs %q, English has %q", lang, key, verbs(msg), verbs(en))
			}
		}
		for key := range msgs {
			if _, ok := catalog[English][key]; !ok {
				t.Errorf("%s: %q is not an English key", lang, key)
			}
		}
	}
}

func verbs(s string) string {
	var out strings.Builder
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' {
			out.WriteByte(s[i+1])
			i++
		}
	}
	return out.String()
}

func TestErrorsFollowTheLanguage(t *testing.T) {
	defer SetLanguage(English)
	sentinel := New("sso.login_required")
	// Our own errors are translated when displayed. (fmt.Errorf, in contrast,
	// fixes its text when it is created.)
	wrapped := Wrap("app.writing_credentials", sentinel)

	SetLanguage(Portuguese)
	if !strings.Contains(wrapped.Error(), "Não foi possível gravar") {
		t.Errorf("pt-BR: %q", wrapped.Error())
	}
	SetLanguage(English)
	if !strings.Contains(wrapped.Error(), "Couldn't write the credentials") {
		t.Errorf("en: %q", wrapped.Error())
	}
	if !errors.Is(fmt.Errorf("outer: %w", wrapped), sentinel) {
		t.Error("errors.Is must see the sentinel through Wrap and fmt.Errorf")
	}

	SetLanguage("xx")
	if Language() != English {
		t.Errorf("unknown language should fall back to English, got %q", Language())
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("missing key = %q", got)
	}
}
