// SPDX-License-Identifier: Apache-2.0

package role_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/framework/goncierge"

	"github.com/gopherium/gophenberg/internal/role"
)

func TestCoreGrantsEachCoreRoleItsCapabilities(t *testing.T) {
	t.Parallel()

	registry := goncierge.New()
	err := registry.Replace(role.Source, role.Core())

	if err != nil {
		t.Fatalf("Replace(%q, Core()) = %v, want the core's names accepted", role.Source, err)
	}
	tests := map[string][]string{
		role.Admin: {
			string(role.ChangeOthersWork), string(role.ManageSettings), string(role.ManageThemes),
			string(role.ManageTypes), string(role.ManageUsers),
		},
		role.Editor: {string(role.ChangeOthersWork)},
		role.Author: nil,
	}
	for named, want := range tests {
		if held := registry.CapabilitiesOf(named); !slices.Equal(held, want) {
			t.Errorf("CapabilitiesOf(%q) = %v, want %v", named, held, want)
		}
	}
	if roles := registry.Roles(); !slices.Equal(roles, []string{role.Admin, role.Author, role.Editor}) {
		t.Errorf("Roles() = %v, want the three core roles", roles)
	}
	if held := registry.HoldersOf(string(role.ManageUsers)); !slices.Equal(held, []string{role.Admin}) {
		t.Errorf("HoldersOf(ManageUsers) = %v, want admin alone", held)
	}
}

func TestRegistryHoldsTheCoreRolesAlone(t *testing.T) {
	t.Parallel()

	registry := role.Registry()

	if roles := registry.Roles(); !slices.Equal(roles, []string{role.Admin, role.Author, role.Editor}) {
		t.Errorf("Roles() = %v, want the three core roles alone", roles)
	}
	if held := registry.HoldersOf(string(role.ManageUsers)); !slices.Equal(held, []string{role.Admin}) {
		t.Errorf("HoldersOf(ManageUsers) = %v, want admin alone", held)
	}
	editorChanges := registry.Can(role.Editor, string(role.ChangeOthersWork))
	if !editorChanges || registry.Can(role.Author, string(role.ChangeOthersWork)) {
		t.Error("the registry does not carry the core's table, want editor to change others' work and author not")
	}
}

func TestCoreCannotBeChangedByItsCaller(t *testing.T) {
	t.Parallel()

	core := role.Core()
	core[0].Capabilities[0] = "manage_everything"
	core[1].Name = "owner"

	again := role.Core()

	if again[0].Capabilities[0] == "manage_everything" || again[1].Name == "owner" {
		t.Errorf("Core() = %v after a caller wrote to it, want the table untouched", again)
	}
}

func TestKnownListsTheRolesInDeclarationOrder(t *testing.T) {
	t.Parallel()

	known := role.Known()

	if want := []string{role.Admin, role.Editor, role.Author}; !slices.Equal(known, want) {
		t.Errorf("Known() = %v, want %v", known, want)
	}
}

func TestKnownCannotBeChangedByItsCaller(t *testing.T) {
	t.Parallel()

	known := role.Known()
	known[0] = "owner"

	if again := role.Known(); again[0] != role.Admin {
		t.Errorf("Known() = %v after a caller wrote to it, want %q first", again, role.Admin)
	}
}

func TestPrivilegedNamesAdminAlone(t *testing.T) {
	t.Parallel()

	held := role.Privileged()

	if !held.Holds(role.Admin) {
		t.Errorf("Privileged() = %v, want it to hold %q", held, role.Admin)
	}
	for _, unprivileged := range []string{role.Editor, role.Author, ""} {
		if held.Holds(unprivileged) {
			t.Errorf("Privileged() holds %q, want the administration surface closed to it", unprivileged)
		}
	}
}

func TestPrivilegedCannotBeChangedByItsCaller(t *testing.T) {
	t.Parallel()

	held := role.Privileged()
	held[0] = role.Author

	if again := role.Privileged(); !again.Holds(role.Admin) {
		t.Errorf("Privileged() = %v after a caller wrote to it, want %q untouched", again, role.Admin)
	}
}

func TestCan(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		role       string
		capability role.Capability
		want       bool
	}{
		"an admin administers accounts":           {role.Admin, role.ManageUsers, true},
		"an admin manages themes":                 {role.Admin, role.ManageThemes, true},
		"an admin reshapes the content model":     {role.Admin, role.ManageTypes, true},
		"an admin writes the site settings":       {role.Admin, role.ManageSettings, true},
		"an admin changes work it did not write":  {role.Admin, role.ChangeOthersWork, true},
		"an editor changes work it did not write": {role.Editor, role.ChangeOthersWork, true},
		"an editor cannot administer accounts":    {role.Editor, role.ManageUsers, false},
		"an editor cannot manage themes":          {role.Editor, role.ManageThemes, false},
		"an author holds none of them":            {role.Author, role.ChangeOthersWork, false},
		"an unknown role holds nothing":           {"archivist", role.ManageUsers, false},
		"an account holding no role holds none":   {"", role.ManageSettings, false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := role.Can(test.role, test.capability); got != test.want {
				t.Errorf("Can(%q, %q) = %v, want %v", test.role, test.capability, got, test.want)
			}
		})
	}
}

func TestPrivilegedIsWhoeverAdministersAccounts(t *testing.T) {
	t.Parallel()

	held := role.Privileged()

	for _, named := range []string{role.Admin, role.Editor, role.Author} {
		if held.Holds(named) != role.Can(named, role.ManageUsers) {
			t.Errorf("Privileged().Holds(%q) = %v, want it to follow Can(%q, ManageUsers)",
				named, held.Holds(named), named)
		}
	}
}

func TestCapabilitiesOf(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		role string
		want []string
	}{
		"an admin carries every capability": {role.Admin, []string{
			string(role.ChangeOthersWork),
			string(role.ManageSettings),
			string(role.ManageThemes),
			string(role.ManageTypes),
			string(role.ManageUsers),
		}},
		"an editor carries only the one it works with": {role.Editor, []string{string(role.ChangeOthersWork)}},
		"an author carries none":                       {role.Author, []string{}},
		"an unknown role carries none":                 {"archivist", []string{}},
		"an account holding no role carries none":      {"", []string{}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := role.CapabilitiesOf(test.role)
			slices.Sort(held)

			if !slices.Equal(held, test.want) {
				t.Errorf("CapabilitiesOf(%q) = %v, want %v", test.role, held, test.want)
			}
		})
	}
}

func TestCapabilitiesOfAnswersEveryCapabilityCanAgreesWith(t *testing.T) {
	t.Parallel()

	every := []role.Capability{
		role.ManageUsers, role.ManageThemes, role.ManageTypes, role.ManageSettings, role.ChangeOthersWork,
	}

	for _, named := range []string{role.Admin, role.Editor, role.Author, "archivist", ""} {
		held := role.CapabilitiesOf(named)
		for _, capability := range every {
			if slices.Contains(held, string(capability)) != role.Can(named, capability) {
				t.Errorf("CapabilitiesOf(%q) and Can(%q, %q) disagree", named, named, capability)
			}
		}
	}
}

func TestCapabilitiesOfCannotBeChangedByItsCaller(t *testing.T) {
	t.Parallel()

	held := role.CapabilitiesOf(role.Editor)
	held[0] = "manage_everything"

	if again := role.CapabilitiesOf(role.Editor); !slices.Equal(again, []string{string(role.ChangeOthersWork)}) {
		t.Errorf("CapabilitiesOf(Editor) = %v after a caller wrote to it, want the table untouched", again)
	}
}

func TestMayChange(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	stranger := uuid.New()
	changes := []string{string(role.ManageUsers), string(role.ChangeOthersWork)}
	tests := map[string]struct {
		capabilities []string
		actor        uuid.UUID
		want         bool
	}{
		"an account changing others' work changes another's":     {changes, stranger, true},
		"an account carrying nothing changes its own work":       {nil, owner, true},
		"an account carrying nothing is refused another's":       {nil, stranger, false},
		"an account carrying other capabilities is refused":      {[]string{string(role.ManageThemes)}, stranger, false},
		"an account carrying a plugin's capability is refused":   {[]string{"notes.manage"}, stranger, false},
		"an account changing others' work still changes its own": {changes, owner, true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := role.MayChange(test.capabilities, test.actor, owner); got != test.want {
				t.Errorf("MayChange(%v, actor, owner) = %v, want %v", test.capabilities, got, test.want)
			}
		})
	}
}
