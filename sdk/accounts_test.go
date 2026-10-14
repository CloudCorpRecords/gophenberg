// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

// siteAccounts is an account reader the way a site builds one, sealed by HostOnly.
type siteAccounts struct {
	HostOnly
}

// People answers each asked id with an account named Maria Perez.
func (siteAccounts) People(_ context.Context, ids []ID, avatarSize int) (map[ID]Person, error) {
	people := make(map[ID]Person, len(ids))
	for _, id := range ids {
		people[id] = Person{ID: id, Name: "Maria Perez", Role: "member", Avatar: string(rune('0' + avatarSize%10))}
	}
	return people, nil
}

func TestASiteLendsAnAccountReaderByEmbeddingHostOnly(t *testing.T) {
	t.Parallel()

	deps := Deps{Accounts: siteAccounts{}}
	deps.Accounts.hostOnly()
	asked := ID{1}

	people, err := deps.Accounts.People(context.Background(), []ID{asked}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if got := people[asked]; got != (Person{ID: asked, Name: "Maria Perez", Role: "member", Avatar: "8"}) {
		t.Errorf("People() = %+v, want the asked account by its id at the asked avatar size", people)
	}
}

func TestPersonCarriesOnlyThePublicFaceOfAnAccount(t *testing.T) {
	t.Parallel()

	var names []string
	for _, field := range reflect.VisibleFields(reflect.TypeFor[Person]()) {
		names = append(names, field.Name)
	}
	if want := []string{"ID", "Name", "Role", "Avatar", "Disabled"}; !slices.Equal(names, want) {
		t.Errorf("Person holds %v, want %v and never an email address", names, want)
	}
}
