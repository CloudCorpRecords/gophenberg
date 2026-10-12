// SPDX-License-Identifier: Apache-2.0

package contenttest

import "testing"

// siteSettingCases are the cases over reading and writing the values a site chooses for itself.
var siteSettingCases = []Case{
	{"ReadsWhatItWrote", siteSettingReadsWhatItWrote},
	{"ReportsAnUnsetKeyAsUnset", siteSettingReportsAnUnsetKeyAsUnset},
	{"TellsAnEmptyValueFromAnUnsetKey", siteSettingTellsAnEmptyValueFromAnUnsetKey},
	{"OverwritesAnExistingKey", siteSettingOverwritesAnExistingKey},
	{"KeepsKeysApart", siteSettingKeepsKeysApart},
	{"StoresSeveralKeysAsOneWrite", siteSettingStoresSeveralKeysAsOneWrite},
	{"ClearsAKey", siteSettingClearsAKey},
}

// siteSettingReadsWhatItWrote reads back as found the value it stored under a key.
func siteSettingReadsWhatItWrote(t *testing.T, s Stores) {
	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, found, err := s.SiteSettings.Lookup(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found {
		t.Error("the key reads as unset, want it found")
	}
	if got != "aurora" {
		t.Errorf("got %q, want the stored value", got)
	}
}

// siteSettingReportsAnUnsetKeyAsUnset reads a key nobody stored as unset and empty, without an error.
func siteSettingReportsAnUnsetKeyAsUnset(t *testing.T, s Stores) {
	got, found, err := s.SiteSettings.Lookup(t.Context(), "theme.active")

	if err != nil {
		t.Fatalf("want an unset key to read cleanly, got %v", err)
	}
	if found {
		t.Error("the key reads as found, want it unset")
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// siteSettingTellsAnEmptyValueFromAnUnsetKey reads a key stored as empty as found.
func siteSettingTellsAnEmptyValueFromAnUnsetKey(t *testing.T, s Stores) {
	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.previous": ""}); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, found, err := s.SiteSettings.Lookup(t.Context(), "theme.previous")

	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found {
		t.Error("a key stored as empty reads as unset, want it found")
	}
	if got != "" {
		t.Errorf("got %q, want the empty value it was set to", got)
	}
}

// siteSettingOverwritesAnExistingKey reads back the later of two values written to one key.
func siteSettingOverwritesAnExistingKey(t *testing.T, s Stores) {
	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.active": "driftwood"}); err != nil {
		t.Fatalf("set the first value: %v", err)
	}

	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err != nil {
		t.Fatalf("set the second value: %v", err)
	}

	got, _, err := s.SiteSettings.Lookup(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != "aurora" {
		t.Errorf("got %q, want the newer value", got)
	}
}

// siteSettingKeepsKeysApart leaves one key as it was when a write names another.
func siteSettingKeepsKeysApart(t *testing.T, s Stores) {
	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err != nil {
		t.Fatalf("set the active theme: %v", err)
	}

	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.previous": "driftwood"}); err != nil {
		t.Fatalf("set the previous theme: %v", err)
	}

	active, _, err := s.SiteSettings.Lookup(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("looking up the active theme: %v", err)
	}
	if active != "aurora" {
		t.Errorf("got %q, want the other key untouched", active)
	}
}

// siteSettingStoresSeveralKeysAsOneWrite stores every value of a write that names several keys.
func siteSettingStoresSeveralKeysAsOneWrite(t *testing.T, s Stores) {
	err := s.SiteSettings.Save(t.Context(), map[string]string{
		"theme.active":   "aurora",
		"theme.previous": "driftwood",
	})

	if err != nil {
		t.Fatalf("save: %v", err)
	}
	for key, want := range map[string]string{"theme.active": "aurora", "theme.previous": "driftwood"} {
		got, found, err := s.SiteSettings.Lookup(t.Context(), key)
		if err != nil {
			t.Fatalf("looking up %s: %v", key, err)
		}
		if !found || got != want {
			t.Errorf("%s = %q (found %v), want %q", key, got, found, want)
		}
	}
}

// siteSettingClearsAKey reads a key back as empty once an empty value is written over it.
func siteSettingClearsAKey(t *testing.T, s Stores) {
	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := s.SiteSettings.Save(t.Context(), map[string]string{"theme.active": ""}); err != nil {
		t.Fatalf("clear: %v", err)
	}

	got, _, err := s.SiteSettings.Lookup(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want the key cleared", got)
	}
}
