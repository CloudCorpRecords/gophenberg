// SPDX-License-Identifier: Apache-2.0

package accountbridge_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/sdk"
)

// countedUsers holds accounts and records every read of many ids.
type countedUsers struct {
	held  []gouncer.User
	asked [][]uuid.UUID
	err   error
}

// UsersByIDs returns the held accounts carrying the ids, recording the read.
func (c *countedUsers) UsersByIDs(_ context.Context, ids []uuid.UUID) ([]gouncer.User, error) {
	c.asked = append(c.asked, ids)
	if c.err != nil {
		return nil, c.err
	}
	var found []gouncer.User
	for _, user := range c.held {
		if slices.Contains(ids, user.ID) {
			found = append(found, user)
		}
	}
	return found, nil
}

// stalledUsers answers only once the read's context ends.
type stalledUsers struct{}

// UsersByIDs waits for the context to end and returns its error.
func (stalledUsers) UsersByIDs(ctx context.Context, _ []uuid.UUID) ([]gouncer.User, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// confirmed returns a confirmed account with the name, email and role.
func confirmed(name, email, role string) gouncer.User {
	return gouncer.User{ID: uuid.New(), Name: name, Email: email, Role: role, Confirmed: true}
}

// sizedAvatar draws an avatar address naming the account's name and the asked size.
func sizedAvatar(name, _ string, size int) string {
	return fmt.Sprintf("avatar:%s:%d", name, size)
}

// openReader returns a reader over users, opened, drawing avatars with sizedAvatar.
func openReader(users accountbridge.Users) *accountbridge.Reader {
	reader := accountbridge.New(users, accountbridge.Options{Avatar: sizedAvatar, Timeout: time.Second})
	reader.Open()
	return reader
}

// idOf returns the sdk id of the account.
func idOf(user gouncer.User) sdk.ID {
	return sdk.ID(user.ID)
}

func TestPeopleReturnsEachAskedAccountByID(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	author := confirmed("Second Author", "second@example.com", "author")
	reader := openReader(&countedUsers{held: []gouncer.User{member, author}})

	people, err := reader.People(context.Background(), []sdk.ID{idOf(member), idOf(author)}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	want := map[sdk.ID]sdk.Person{
		idOf(member): {ID: idOf(member), Name: "Maria Perez", Role: "member", Avatar: "avatar:Maria Perez:48"},
		idOf(author): {ID: idOf(author), Name: "Second Author", Role: "author", Avatar: "avatar:Second Author:48"},
	}
	if fmt.Sprint(people) != fmt.Sprint(want) {
		t.Errorf("People() = %+v, want %+v", people, want)
	}
}

func TestPeopleReadsOnceForManyIDs(t *testing.T) {
	t.Parallel()

	held := []gouncer.User{
		confirmed("Maria Perez", "maria@example.com", "member"),
		confirmed("Second Author", "second@example.com", "author"),
		confirmed("Third Editor", "third@example.com", "editor"),
	}
	users := &countedUsers{held: held}

	if _, err := openReader(users).People(
		context.Background(), []sdk.ID{idOf(held[0]), idOf(held[1]), idOf(held[2])}, 48,
	); err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if len(users.asked) != 1 || len(users.asked[0]) != 3 {
		t.Errorf("the store was read %d times with %v, want one read of the three ids", len(users.asked), users.asked)
	}
}

func TestPeopleLeavesOutAnIDNoAccountHolds(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	unknown := sdk.ID(uuid.New())

	people, err := openReader(&countedUsers{held: []gouncer.User{member}}).People(
		context.Background(), []sdk.ID{idOf(member), unknown}, 48)
	if err != nil {
		t.Fatalf("People() error = %v, want no error for an id no account holds", err)
	}
	if _, found := people[unknown]; found || len(people) != 1 {
		t.Errorf("People() = %+v, want only the held account", people)
	}
}

func TestPeopleReadsARepeatedIDOnce(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	users := &countedUsers{held: []gouncer.User{member}}

	people, err := openReader(users).People(context.Background(), []sdk.ID{idOf(member), idOf(member)}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if len(people) != 1 || len(users.asked) != 1 || len(users.asked[0]) != 1 {
		t.Errorf("People() = %+v after reading %v, want one account from one read of one id", people, users.asked)
	}
}

func TestPeopleWithNoIDsReadsNothing(t *testing.T) {
	t.Parallel()

	users := &countedUsers{}

	people, err := openReader(users).People(context.Background(), nil, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if people == nil || len(people) != 0 || len(users.asked) != 0 {
		t.Errorf("People() = %#v after %d reads, want an empty map and no read", people, len(users.asked))
	}
}

func TestPeopleMarksADisabledAccount(t *testing.T) {
	t.Parallel()

	disabled := confirmed("Disabled Member", "disabled@example.com", "member")
	disabled.Disabled = true

	people, err := openReader(&countedUsers{held: []gouncer.User{disabled}}).People(
		context.Background(), []sdk.ID{idOf(disabled)}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if got := people[idOf(disabled)]; got.Name != "Disabled Member" || !got.Disabled {
		t.Errorf("People() = %+v, want the disabled account by name, marked disabled", people)
	}
}

func TestPeopleLeavesOutAnUnconfirmedAccount(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	unconfirmed := confirmed("Unconfirmed Member", "unconfirmed@example.com", "member")
	unconfirmed.Confirmed = false

	people, err := openReader(&countedUsers{held: []gouncer.User{member, unconfirmed}}).People(
		context.Background(), []sdk.ID{idOf(member), idOf(unconfirmed)}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if _, found := people[idOf(unconfirmed)]; found || len(people) != 1 {
		t.Errorf("People() = %+v, want the unconfirmed account left out", people)
	}
}

func TestPeopleDrawsAvatarsFromTheNameAndEmailAtTheAskedSize(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	var drawn []string
	reader := accountbridge.New(&countedUsers{held: []gouncer.User{member}}, accountbridge.Options{
		Avatar: func(name, email string, size int) string {
			drawn = append(drawn, fmt.Sprintf("%s %s %d", name, email, size))
			return "drawn"
		},
		Timeout: time.Second,
	})
	reader.Open()

	people, err := reader.People(context.Background(), []sdk.ID{idOf(member)}, 96)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if people[idOf(member)].Avatar != "drawn" || !slices.Equal(drawn, []string{"Maria Perez maria@example.com 96"}) {
		t.Errorf("People() = %+v after drawing %v, want the avatar drawn once from the name and email at 96", people, drawn)
	}
}

func TestPeopleTakesAvatarSizesFromOneTo2048(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	reader := openReader(&countedUsers{held: []gouncer.User{member}})

	for _, size := range []int{1, 2048} {
		if _, err := reader.People(context.Background(), []sdk.ID{idOf(member)}, size); err != nil {
			t.Errorf("People() at %d pixels error = %v, want it drawn", size, err)
		}
	}
}

func TestPeopleRefusesAnAvatarSizeOutsideTheRange(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")

	for _, size := range []int{0, -1, 2049} {
		users := &countedUsers{held: []gouncer.User{member}}
		people, err := openReader(users).People(context.Background(), []sdk.ID{idOf(member)}, size)
		if !errors.Is(err, accountbridge.ErrAvatarSize) || people != nil || len(users.asked) != 0 {
			t.Errorf("People() at %d pixels = %v, %v after %d reads, want ErrAvatarSize before any read",
				size, people, err, len(users.asked))
		}
		if err != nil && !strings.Contains(err.Error(), fmt.Sprint(size)) {
			t.Errorf("People() at %d pixels error = %q, want the refused size named", size, err)
		}
	}
}

func TestPeopleNeverPutsAnEmailInTheAnswer(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	author := confirmed("Second Author", "second@example.com", "author")

	people, err := openReader(&countedUsers{held: []gouncer.User{member, author}}).People(
		context.Background(), []sdk.ID{idOf(member), idOf(author)}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if answer := fmt.Sprintf("%+v", people); strings.Contains(answer, "@") {
		t.Errorf("People() = %s, want no email address in the answer", answer)
	}
}

func TestPeopleRefusesUntilTheSiteOpensIt(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	users := &countedUsers{held: []gouncer.User{member}}
	reader := accountbridge.New(users, accountbridge.Options{Avatar: sizedAvatar, Timeout: time.Second})

	people, err := reader.People(context.Background(), []sdk.ID{idOf(member)}, 48)
	if !errors.Is(err, accountbridge.ErrNotOpen) || people != nil || len(users.asked) != 0 {
		t.Errorf("People() before Open = %v, %v after %d reads, want ErrNotOpen before any read",
			people, err, len(users.asked))
	}
}

func TestPeopleWrapsAReadFailure(t *testing.T) {
	t.Parallel()

	refused := errors.New("the store is unreachable")

	people, err := openReader(&countedUsers{err: refused}).People(
		context.Background(), []sdk.ID{sdk.ID(uuid.New())}, 48)
	if !errors.Is(err, refused) || people != nil {
		t.Errorf("People() = %v, %v, want no map and the store's error wrapped", people, err)
	}
}

func TestPeopleEndsTheReadAtTheTimeout(t *testing.T) {
	t.Parallel()

	reader := accountbridge.New(stalledUsers{}, accountbridge.Options{Avatar: sizedAvatar, Timeout: time.Millisecond})
	reader.Open()

	_, err := reader.People(context.Background(), []sdk.ID{sdk.ID(uuid.New())}, 48)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("People() error = %v, want the read ended at the timeout", err)
	}
}
