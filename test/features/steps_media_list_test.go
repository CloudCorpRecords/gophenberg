// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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

// theImageCarriesThe gives the stored item titled title the words as its caption or its description.
func theImageCarriesThe(ctx context.Context, title, field, words string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.mediaStore.changeTitled(title, func(item *media.Media) {
		if field == "caption" {
			item.Caption = words
			return
		}
		item.Description = words
	})
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

// dateRange reads a range a scenario names, `after "D"`, `before "D"` or both, into its bounds.
var dateRange = regexp.MustCompile(`^(?:after "(\d{4}-\d{2}-\d{2})")?(?:(?: and )?before "(\d{4}-\d{2}-\d{2})")?$`)

// filtersTheLibraryToTheUploadsOfThatAuthor lists only the uploads of the other author.
func filtersTheLibraryToTheUploadsOfThatAuthor(ctx context.Context) error {
	return listsTheUploadsOfAnotherAuthor(ctx, "author")
}

// leavesTheUploadsOfThatAuthorOut lists the library without the uploads of the other author.
func leavesTheUploadsOfThatAuthorOut(ctx context.Context) error {
	return listsTheUploadsOfAnotherAuthor(ctx, "author_exclude")
}

// listsTheUploadsOfAnotherAuthor lists the library naming the other author's account under the parameter.
func listsTheUploadsOfAnotherAuthor(ctx context.Context, parameter string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	author, err := w.users.UserByEmail(ctx, anotherAuthorEmail)
	if err != nil {
		return fmt.Errorf("finding another author: %w", err)
	}
	return w.get(mediaPath + "?" + url.Values{parameter: {author.ID.String()}}.Encode())
}

// filtersTheLibraryToTheItemsAdded lists the items added within the range, on the before and after bounds.
func filtersTheLibraryToTheItemsAdded(ctx context.Context, between string) error {
	return listsWithinTheRange(ctx, between, "after", "before")
}

// filtersTheLibraryToTheItemsModified lists the items last modified within the range.
func filtersTheLibraryToTheItemsModified(ctx context.Context, between string) error {
	return listsWithinTheRange(ctx, between, "modified_after", "modified_before")
}

// listsWithinTheRange lists the library with the range's days, at midnight UTC, under the two bound parameters.
func listsWithinTheRange(ctx context.Context, between, afterParameter, beforeParameter string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	bounds := dateRange.FindStringSubmatch(between)
	if bounds == nil {
		return fmt.Errorf("no range reads %s", between)
	}
	query := url.Values{}
	for parameter, day := range map[string]string{afterParameter: bounds[1], beforeParameter: bounds[2]} {
		if day != "" {
			query.Set(parameter, day+"T00:00:00Z")
		}
	}
	return w.get(mediaPath + "?" + query.Encode())
}

// theLibraryListsOnly asserts the last listing carries exactly the titles, in any order.
func theLibraryListsOnly(ctx context.Context, names string) error {
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
	want := quotedTitles(names)
	slices.Sort(titles)
	slices.Sort(want)
	if !slices.Equal(titles, want) {
		return fmt.Errorf("the library lists %q, want only %q", titles, want)
	}
	return nil
}

// registerMediaListSteps binds the steps that sort and narrow the media library.
func registerMediaListSteps(sc *godog.ScenarioContext) {
	sc.Given(`^"([^"]*)" was added and last modified on "([^"]*)"$`, wasAddedAndLastModifiedOn)
	sc.Given(`^"([^"]*)" was added on "([^"]*)" and last modified on "([^"]*)"$`, wasAddedOnAndLastModifiedOn)
	sc.Given(`^"([^"]*)" and "([^"]*)" were modified at the same moment$`, wereModifiedAtTheSameMoment)
	sc.Given(`^"([^"]*)" was modified after them$`, wasModifiedAfterThem)
	sc.Given(`^"([^"]*)" was uploaded by an author whose name sorts first$`, wasUploadedByAnAuthorWhoseNameSortsFirst)
	sc.Given(`^the image "([^"]*)" carries the (caption|description) "([^"]*)"$`, theImageCarriesThe)
	sc.When(`^the administrator lists the library sorted by "([^"]*)" "([^"]*)"$`, listsTheLibrarySortedByIn)
	sc.When(`^the administrator lists the library sorted by "([^"]*)"$`, listsTheLibrarySortedBy)
	sc.When(`^the administrator filters the library to the uploads of that author$`,
		filtersTheLibraryToTheUploadsOfThatAuthor)
	sc.When(`^the administrator leaves the uploads of that author out of the library$`, leavesTheUploadsOfThatAuthorOut)
	sc.When(`^the administrator filters the library to the items added (.+)$`, filtersTheLibraryToTheItemsAdded)
	sc.When(`^the administrator filters the library to the items modified (.+)$`, filtersTheLibraryToTheItemsModified)
	sc.Then(`^the library lists ((?:"[^"]*"(?:, | and )?)+) in that order$`, theLibraryListsInThatOrder)
	sc.Then(`^the library lists only ((?:"[^"]*"(?:, | and )?)+)$`, theLibraryListsOnly)
}
