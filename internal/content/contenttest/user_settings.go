// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"

	"github.com/google/uuid"
)

// userSettingCases are the cases over reading and writing the values each reader chooses for themselves.
var userSettingCases = []Case{
	{"ReadsWhatItWrote", userSettingReadsWhatItWrote},
	{"CarriesTheLatestWrite", userSettingCarriesTheLatestWrite},
	{"IsTheReadersAlone", userSettingIsTheReadersAlone},
	{"AnswersNothingForAKeyItNeverHeld", userSettingAnswersNothingForAKeyItNeverHeld},
	{"KeepsEachKeyApart", userSettingKeepsEachKeyApart},
	{"MatchesKeysExactly", userSettingMatchesKeysExactly},
}

// userSettingReadsWhatItWrote reads back as found the value a reader stored under a key.
func userSettingReadsWhatItWrote(t *testing.T, s Stores) {
	reader := uuid.New()

	if err := s.UserSettings.Save(t.Context(), reader, "locale", "es-ES"); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	held, found, err := s.UserSettings.Lookup(t.Context(), reader, "locale")

	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if !found || held != "es-ES" {
		t.Errorf("Lookup() = %q, %v, want the language the reader chose", held, found)
	}
}

// userSettingCarriesTheLatestWrite reads back the value a reader wrote last under a key.
func userSettingCarriesTheLatestWrite(t *testing.T, s Stores) {
	reader := uuid.New()
	if err := s.UserSettings.Save(t.Context(), reader, "locale", "es-ES"); err != nil {
		t.Fatalf("the first Save() error = %v, want nil", err)
	}

	if err := s.UserSettings.Save(t.Context(), reader, "locale", "fr-FR"); err != nil {
		t.Fatalf("the second Save() error = %v, want nil", err)
	}

	held, _, err := s.UserSettings.Lookup(t.Context(), reader, "locale")
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if held != "fr-FR" {
		t.Errorf("Lookup() = %q, want the language written last", held)
	}
}

// userSettingIsTheReadersAlone answers the value one reader stored to that reader alone.
func userSettingIsTheReadersAlone(t *testing.T, s Stores) {
	mine, theirs := uuid.New(), uuid.New()
	if err := s.UserSettings.Save(t.Context(), mine, "locale", "es-ES"); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	_, found, err := s.UserSettings.Lookup(t.Context(), theirs, "locale")

	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if found {
		t.Error("another reader's choice was answered, want each reader answered its own")
	}
	if held, found, err := s.UserSettings.Lookup(t.Context(), mine, "locale"); err != nil || !found || held != "es-ES" {
		t.Errorf("Lookup(mine) = %q, %v, error %v, want the reader's own choice", held, found, err)
	}
}

// userSettingAnswersNothingForAKeyItNeverHeld reads a key no reader stored as unset and empty, without an error.
func userSettingAnswersNothingForAKeyItNeverHeld(t *testing.T, s Stores) {
	held, found, err := s.UserSettings.Lookup(t.Context(), uuid.New(), "never-written")

	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if found || held != "" {
		t.Errorf("Lookup() = %q, %v, want nothing stored", held, found)
	}
}

// userSettingKeepsEachKeyApart holds each key of one reader on its own, through an overwrite of one of them.
func userSettingKeepsEachKeyApart(t *testing.T, s Stores) {
	reader := uuid.New()
	for _, write := range []struct{ key, value string }{
		{"locale", "es-ES"}, {"theme", "aurora"}, {"locale", "fr-FR"},
	} {
		if err := s.UserSettings.Save(t.Context(), reader, write.key, write.value); err != nil {
			t.Fatalf("Save(%q, %q) error = %v, want nil", write.key, write.value, err)
		}
	}

	for key, want := range map[string]string{"locale": "fr-FR", "theme": "aurora"} {
		held, found, err := s.UserSettings.Lookup(t.Context(), reader, key)
		if err != nil {
			t.Fatalf("Lookup(%q) error = %v, want nil", key, err)
		}
		if !found || held != want {
			t.Errorf("Lookup(%q) = %q, %v, want %q", key, held, found, want)
		}
	}
}

// userSettingMatchesKeysExactly finds nothing under a key that differs from a stored one in case or a trailing space.
func userSettingMatchesKeysExactly(t *testing.T, s Stores) {
	reader := uuid.New()
	if err := s.UserSettings.Save(t.Context(), reader, "locale", "es-ES"); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
	if _, found, err := s.UserSettings.Lookup(t.Context(), reader, "locale"); err != nil || !found {
		t.Fatalf("Lookup(%q) = found %v, error %v, want the stored key found", "locale", found, err)
	}

	for _, near := range []string{"Locale", "locale "} {
		held, found, err := s.UserSettings.Lookup(t.Context(), reader, near)
		if err != nil {
			t.Fatalf("Lookup(%q) error = %v, want nil", near, err)
		}
		if found || held != "" {
			t.Errorf("Lookup(%q) = %q, %v, want nothing, as only %q was stored", near, held, found, "locale")
		}
	}
}
