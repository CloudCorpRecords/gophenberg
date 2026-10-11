// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/cucumber/godog"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/role"
)

// The account the scenarios store as an author whose name sorts before every other.
const (
	firstAuthorEmail = "author-sorting-first@example.com"
	firstAuthorName  = "Author Sorting First"
)

// tiedModification is the moment the scenarios give two items as their last modification.
var tiedModification = time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)

// mediaDay reads a day a scenario names as its first moment in UTC.
func mediaDay(day string) (time.Time, error) {
	at, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return time.Time{}, fmt.Errorf("reading the day %q: %w", day, err)
	}
	return at, nil
}

// restamped gives the stored item titled title the moments it was added and last modified.
func restamped(ctx context.Context, title string, added, modified time.Time) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.mediaStore.changeTitled(title, func(item *media.Media) {
		item.CreatedAt, item.UpdatedAt = added, modified
	})
}

// wasAddedAndLastModifiedOn gives the item one day as both its added and its last modified moment.
func wasAddedAndLastModifiedOn(ctx context.Context, title, day string) error {
	at, err := mediaDay(day)
	if err != nil {
		return err
	}
	return restamped(ctx, title, at, at)
}

// wasAddedOnAndLastModifiedOn gives the item the day it was added and the day it was last modified.
func wasAddedOnAndLastModifiedOn(ctx context.Context, title, addedDay, modifiedDay string) error {
	added, err := mediaDay(addedDay)
	if err != nil {
		return err
	}
	modified, err := mediaDay(modifiedDay)
	if err != nil {
		return err
	}
	return restamped(ctx, title, added, modified)
}

// remodified gives the stored item titled title the moment it was last modified.
func remodified(ctx context.Context, title string, at time.Time) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.mediaStore.changeTitled(title, func(item *media.Media) { item.UpdatedAt = at })
}

// wereModifiedAtTheSameMoment gives both items the same last modified moment.
func wereModifiedAtTheSameMoment(ctx context.Context, one, other string) error {
	if err := remodified(ctx, one, tiedModification); err != nil {
		return err
	}
	return remodified(ctx, other, tiedModification)
}

// wasModifiedAfterThem gives the item a last modified moment after the tied pair.
func wasModifiedAfterThem(ctx context.Context, title string) error {
	return remodified(ctx, title, tiedModification.Add(24*time.Hour))
}

// wasUploadedByAnAuthorWhoseNameSortsFirst files the item under an account whose name sorts before every other.
func wasUploadedByAnAuthorWhoseNameSortsFirst(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	_, err = authkit.EnsureAdmin(ctx, w.users, firstAuthorEmail, firstAuthorName, adminPassword, role.Author)
	if err != nil {
		return fmt.Errorf("storing the author sorting first: %w", err)
	}
	author, err := w.users.UserByEmail(ctx, firstAuthorEmail)
	if err != nil {
		return fmt.Errorf("finding the author sorting first: %w", err)
	}
	return w.mediaStore.changeTitled(title, func(item *media.Media) { item.AuthorID = author.ID })
}

// listsTheLibrarySortedByIn lists the library sorted by the column in the direction.
func listsTheLibrarySortedByIn(ctx context.Context, orderBy, order string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "?" + url.Values{"orderby": {orderBy}, "order": {order}}.Encode())
}

// listsTheLibrarySortedBy lists the library sorted by the column, naming no direction.
func listsTheLibrarySortedBy(ctx context.Context, orderBy string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "?" + url.Values{"orderby": {orderBy}}.Encode())
}

// theLibraryListsInThatOrder asserts the last listing carries exactly the titles, in that order.
func theLibraryListsInThatOrder(ctx context.Context, names string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("listing the library: %w", err)
	}
	var listed struct {
		Items []listedMedia `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	titles := make([]string, len(listed.Items))
	for i, item := range listed.Items {
		titles[i] = item.Title
	}
	if want := quotedTitles(names); !slices.Equal(titles, want) {
		return fmt.Errorf("the library lists %q, want %q", titles, want)
	}
	return nil
}

// registerMediaListSteps binds the steps that sort the media library.
func registerMediaListSteps(sc *godog.ScenarioContext) {
	sc.Given(`^"([^"]*)" was added and last modified on "([^"]*)"$`, wasAddedAndLastModifiedOn)
	sc.Given(`^"([^"]*)" was added on "([^"]*)" and last modified on "([^"]*)"$`, wasAddedOnAndLastModifiedOn)
	sc.Given(`^"([^"]*)" and "([^"]*)" were modified at the same moment$`, wereModifiedAtTheSameMoment)
	sc.Given(`^"([^"]*)" was modified after them$`, wasModifiedAfterThem)
	sc.Given(`^"([^"]*)" was uploaded by an author whose name sorts first$`, wasUploadedByAnAuthorWhoseNameSortsFirst)
	sc.When(`^the administrator lists the library sorted by "([^"]*)" "([^"]*)"$`, listsTheLibrarySortedByIn)
	sc.When(`^the administrator lists the library sorted by "([^"]*)"$`, listsTheLibrarySortedBy)
	sc.Then(`^the library lists ((?:"[^"]*"(?:, | and )?)+) in that order$`, theLibraryListsInThatOrder)
}
