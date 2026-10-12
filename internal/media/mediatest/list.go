// SPDX-License-Identifier: Apache-2.0

package mediatest

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// listCases are the cases over listing, sorting, narrowing, searching and paging the media library.
var listCases = []Case{
	{"ListsNewestFirst", listsNewestFirst},
	{"ListSortsAsTheFilterNames", listSortsAsTheFilterNames},
	{"ListNarrowsByAuthor", listNarrowsByAuthor},
	{"ListNarrowsByTheDatesAddedAndModified", listNarrowsByTheDatesAddedAndModified},
	{"ListBreaksATieByTheUploadDateBeforeTheInsertOrder", listBreaksATieByTheUploadDateBeforeTheInsertOrder},
	{"ListBreaksATieNewestUploadFirst", listBreaksATieNewestUploadFirst},
	{"ListNamesTheAuthorOfEveryRow", listNamesTheAuthorOfEveryRow},
	{"ListFiltersByType", listFiltersByType},
	{"ListFiltersByContentTypePrefix", listFiltersByContentTypePrefix},
	{"ListSearchesTitleAndFile", listSearchesTitleAndFile},
	{"ListSearchesCaptionAndDescription", listSearchesCaptionAndDescription},
	{"ListTreatsPatternCharactersAsText", listTreatsPatternCharactersAsText},
	{"ListPaginates", listPaginates},
	{"ListSortsTitlesAndAuthorsWithoutRegardToCase", listSortsTitlesAndAuthorsWithoutRegardToCase},
	{"ListSearchesWithoutRegardToCaseBeyondASCII", listSearchesWithoutRegardToCaseBeyondASCII},
}

// mustCreateMediaAt stores an image under the author, added and last modified at the given moments.
func mustCreateMediaAt(t *testing.T, store media.Store, title string, author uuid.UUID, added, modified time.Time) {
	t.Helper()
	item := MustImage(t, "2026/08/"+title+".jpg", title, author)
	item.CreatedAt, item.UpdatedAt = added, modified
	MustCreate(t, store, item)
}

// listedTitles returns the titles of the rows the filter lists, in the order the store answers them.
func listedTitles(t *testing.T, store media.Store, f media.Filter) []string {
	t.Helper()
	f.Page, f.PerPage = 1, 10
	items, _, err := store.List(t.Context(), f)
	if err != nil {
		t.Fatalf("List(%q %q) error = %v, want nil", f.OrderBy, f.Order, err)
	}
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Title
	}
	return titles
}

// narrowedTitles returns the titles of the rows the filter lists, failing when the total differs from the rows.
func narrowedTitles(t *testing.T, store media.Store, f media.Filter) []string {
	t.Helper()
	f.Page, f.PerPage = 1, 10
	items, total, err := store.List(t.Context(), f)
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != len(items) {
		t.Errorf("List() total = %d, want the %d rows it lists", total, len(items))
	}
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Title
	}
	return titles
}

// listsNewestFirst lists the latest upload first when the filter names no order.
func listsNewestFirst(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/older.jpg", "older", author))
	newest := MustCreate(t, s.Media, MustImage(t, "2026/08/newest.jpg", "newest", author))

	items, total, err := s.Media.List(t.Context(), media.Filter{Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("List() = %d items with total %d, want both stored items", len(items), total)
	}
	if items[0].ID != newest.ID {
		t.Errorf("first listed = %q, want the newest upload", items[0].File)
	}
}

// listSortsAsTheFilterNames sorts by the date added, the date modified, the title or the author, either way.
func listSortsAsTheFilterNames(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	sortsFirst := s.AddAuthor(t, "Author Sorting First")
	day := func(month time.Month) time.Time { return time.Date(2026, month, 10, 0, 0, 0, 0, time.UTC) }
	mustCreateMediaAt(t, s.Media, "harbor", author, day(time.July), day(time.July))
	mustCreateMediaAt(t, s.Media, "cliff", sortsFirst, day(time.March), day(time.September))
	mustCreateMediaAt(t, s.Media, "manual", author, day(time.May), day(time.May))

	tests := []struct {
		orderBy media.OrderBy
		order   media.Order
		want    []string
	}{
		{media.OrderByDate, media.OrderAsc, []string{"cliff", "manual", "harbor"}},
		{media.OrderByDate, media.OrderDesc, []string{"harbor", "manual", "cliff"}},
		{media.OrderByModified, media.OrderAsc, []string{"manual", "harbor", "cliff"}},
		{media.OrderByModified, media.OrderDesc, []string{"cliff", "harbor", "manual"}},
		{media.OrderByTitle, media.OrderAsc, []string{"cliff", "harbor", "manual"}},
		{media.OrderByTitle, media.OrderDesc, []string{"manual", "harbor", "cliff"}},
		{media.OrderByAuthor, media.OrderAsc, []string{"cliff", "harbor", "manual"}},
		{media.OrderByAuthor, media.OrderDesc, []string{"harbor", "manual", "cliff"}},
	}
	for _, tt := range tests {
		got := listedTitles(t, s.Media, media.Filter{OrderBy: tt.orderBy, Order: tt.order})

		if !slices.Equal(got, tt.want) {
			t.Errorf("List(%q %q) = %q, want %q", tt.orderBy, tt.order, got, tt.want)
		}
	}
}

// listNarrowsByAuthor keeps the items of the named authors and leaves out the excluded ones.
func listNarrowsByAuthor(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	second := s.AddAuthor(t, "Second Author")
	MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	MustCreate(t, s.Media, MustImage(t, "2026/08/pier.jpg", "pier", second))

	tests := []struct {
		name   string
		filter media.Filter
		want   []string
	}{
		{"one author", media.Filter{Authors: []uuid.UUID{second}}, []string{"pier"}},
		{"both authors", media.Filter{Authors: []uuid.UUID{author, second}}, []string{"pier", "harbor"}},
		{"one author left out", media.Filter{ExcludeAuthors: []uuid.UUID{second}}, []string{"harbor"}},
		{"none named", media.Filter{}, []string{"pier", "harbor"}},
	}
	for _, tt := range tests {
		if got := narrowedTitles(t, s.Media, tt.filter); !slices.Equal(got, tt.want) {
			t.Errorf("%s: List() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// listNarrowsByTheDatesAddedAndModified keeps the items added or modified strictly inside the named bounds.
func listNarrowsByTheDatesAddedAndModified(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	day := func(month time.Month, date int) *time.Time {
		at := time.Date(2026, month, date, 0, 0, 0, 0, time.UTC)
		return &at
	}
	mustCreateMediaAt(t, s.Media, "harbor", author, *day(time.July, 10), *day(time.July, 10))
	mustCreateMediaAt(t, s.Media, "cliff", author, *day(time.March, 10), *day(time.September, 10))
	mustCreateMediaAt(t, s.Media, "manual", author, *day(time.May, 10), *day(time.May, 10))

	tests := []struct {
		name   string
		filter media.Filter
		want   []string
	}{
		{"added before", media.Filter{Before: day(time.June, 1)}, []string{"manual", "cliff"}},
		{"added after", media.Filter{After: day(time.June, 1)}, []string{"harbor"}},
		{"added between", media.Filter{After: day(time.April, 1), Before: day(time.June, 1)}, []string{"manual"}},
		{"added before its own day", media.Filter{Before: day(time.May, 10)}, []string{"cliff"}},
		{"added after its own day", media.Filter{After: day(time.May, 10)}, []string{"harbor"}},
		{"modified before", media.Filter{ModifiedBefore: day(time.June, 1)}, []string{"manual"}},
		{"modified after", media.Filter{ModifiedAfter: day(time.June, 1)}, []string{"harbor", "cliff"}},
		{"modified before its own day", media.Filter{ModifiedBefore: day(time.July, 10)}, []string{"manual"}},
		{"modified after its own day", media.Filter{ModifiedAfter: day(time.July, 10)}, []string{"cliff"}},
	}
	for _, tt := range tests {
		if got := narrowedTitles(t, s.Media, tt.filter); !slices.Equal(got, tt.want) {
			t.Errorf("%s: List() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// listBreaksATieByTheUploadDateBeforeTheInsertOrder lists items of one author newest upload first either way.
func listBreaksATieByTheUploadDateBeforeTheInsertOrder(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	day := func(month time.Month) time.Time { return time.Date(2026, month, 10, 0, 0, 0, 0, time.UTC) }
	mustCreateMediaAt(t, s.Media, "harbor", author, day(time.July), day(time.July))
	mustCreateMediaAt(t, s.Media, "cliff", author, day(time.March), day(time.March))
	mustCreateMediaAt(t, s.Media, "manual", author, day(time.May), day(time.May))

	for order, want := range map[media.Order][]string{
		media.OrderAsc:  {"harbor", "manual", "cliff"},
		media.OrderDesc: {"harbor", "manual", "cliff"},
	} {
		got := listedTitles(t, s.Media, media.Filter{OrderBy: media.OrderByAuthor, Order: order})

		if !slices.Equal(got, want) {
			t.Errorf("List(author %q) = %q, want the newest upload first %q", order, got, want)
		}
	}
}

// listBreaksATieNewestUploadFirst lists items modified at one moment by the latest insert first either way.
func listBreaksATieNewestUploadFirst(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	same := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	later := same.Add(24 * time.Hour)
	mustCreateMediaAt(t, s.Media, "harbor", author, same, same)
	mustCreateMediaAt(t, s.Media, "manual", author, same, same)
	mustCreateMediaAt(t, s.Media, "cliff", author, same, later)

	for order, want := range map[media.Order][]string{
		media.OrderAsc:  {"manual", "harbor", "cliff"},
		media.OrderDesc: {"cliff", "manual", "harbor"},
	} {
		got := listedTitles(t, s.Media, media.Filter{OrderBy: media.OrderByModified, Order: order})

		if !slices.Equal(got, want) {
			t.Errorf("List(modified %q) = %q, want %q", order, got, want)
		}
	}
}

// listNamesTheAuthorOfEveryRow carries the name of each row's own author beside it.
func listNamesTheAuthorOfEveryRow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	second := s.AddAuthor(t, "Second Author")
	MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	MustCreate(t, s.Media, MustImage(t, "2026/08/cliff.jpg", "cliff", second))
	MustCreate(t, s.Media, MustImage(t, "2026/08/pier.jpg", "pier", author))

	items, total, err := s.Media.List(t.Context(), media.Filter{Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 || len(items) != total {
		t.Fatalf("List() = %d items with total %d, want the count equal to the three rows", len(items), total)
	}
	want := map[uuid.UUID]string{author: DefaultAuthor, second: "Second Author"}
	named := make(map[string]bool, len(want))
	for _, item := range items {
		if item.AuthorName != want[item.AuthorID] {
			t.Errorf("%q names its author %q, want %q", item.Title, item.AuthorName, want[item.AuthorID])
		}
		named[item.AuthorName] = true
	}
	if len(named) != len(want) {
		t.Errorf("List() names %d authors, want both", len(named))
	}
}

// listFiltersByType keeps only the images when the filter names that kind and every kind when it names none.
func listFiltersByType(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	manual, err := media.New("2026/08/manual.pdf", "manual", "application/pdf", author)
	if err != nil {
		t.Fatalf("New(manual) error = %v, want nil", err)
	}
	MustCreate(t, s.Media, manual)

	images, imageTotal, err := s.Media.List(t.Context(), media.Filter{Type: media.TypeImage, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List(images) error = %v, want nil", err)
	}
	if imageTotal != 1 || len(images) != 1 || images[0].Type != media.TypeImage {
		t.Errorf("List(images) = %d items with total %d, want only the image", len(images), imageTotal)
	}

	everything, allTotal, err := s.Media.List(t.Context(), media.Filter{Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List(all) error = %v, want nil", err)
	}
	if allTotal != 2 || len(everything) != 2 {
		t.Errorf("List(all) = %d items with total %d, want every kind", len(everything), allTotal)
	}
}

// listFiltersByContentTypePrefix keeps the items whose content type starts with any named prefix, each once.
func listFiltersByContentTypePrefix(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	launch, err := media.New("2026/08/launch.mp4", "launch", "video/mp4", author)
	if err != nil {
		t.Fatalf("New(launch) error = %v, want nil", err)
	}
	MustCreate(t, s.Media, launch)
	chime, err := media.New("2026/08/chime.mp3", "chime", "audio/mpeg", author)
	if err != nil {
		t.Fatalf("New(chime) error = %v, want nil", err)
	}
	MustCreate(t, s.Media, chime)

	videos, total, err := s.Media.List(t.Context(), media.Filter{Mimes: []string{"video"}, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List(video) error = %v, want nil", err)
	}
	if total != 1 || len(videos) != 1 || videos[0].Title != "launch" {
		t.Errorf("List(video) = %d items with total %d, want the video alone", len(videos), total)
	}

	exact, _, err := s.Media.List(t.Context(), media.Filter{Mimes: []string{"audio/mpeg"}, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List(audio/mpeg) error = %v, want nil", err)
	}
	if len(exact) != 1 || exact[0].Title != "chime" {
		t.Errorf("List(audio/mpeg) = %d items, want the audio alone", len(exact))
	}

	pair, pairTotal, err := s.Media.List(
		t.Context(), media.Filter{Mimes: []string{"video", "audio"}, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List(video, audio) error = %v, want nil", err)
	}
	if pairTotal != 2 || len(pair) != 2 {
		t.Errorf("List(video, audio) = %d items with total %d, want both named kinds", len(pair), pairTotal)
	}
	for _, item := range pair {
		if item.Type == media.TypeImage {
			t.Errorf("List(video, audio) carries %q, want no image", item.Title)
		}
	}

	overlapping, overlappingTotal, err := s.Media.List(
		t.Context(), media.Filter{Mimes: []string{"video", "video/mp4"}, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List(video, video/mp4) error = %v, want nil", err)
	}
	if overlappingTotal != 1 || len(overlapping) != 1 {
		t.Errorf("List(video, video/mp4) = %d items with total %d, want the video counted once",
			len(overlapping), overlappingTotal)
	}
}

// listSearchesTitleAndFile finds an item by a word of its title or a part of its file name.
func listSearchesTitleAndFile(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/dsc-0042.jpg", "Harbor at dawn", author))
	MustCreate(t, s.Media, MustImage(t, "2026/08/cliff.jpg", "Cliff", author))

	byTitle, _, err := s.Media.List(t.Context(), media.Filter{Search: "harbor", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List(harbor) error = %v, want nil", err)
	}
	if len(byTitle) != 1 || byTitle[0].Title != "Harbor at dawn" {
		t.Errorf("List(harbor) = %d items, want the title match alone", len(byTitle))
	}

	byFile, _, err := s.Media.List(t.Context(), media.Filter{Search: "dsc-0042", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List(dsc-0042) error = %v, want nil", err)
	}
	if len(byFile) != 1 || byFile[0].File != "2026/08/dsc-0042.jpg" {
		t.Errorf("List(dsc-0042) = %d items, want the file match alone", len(byFile))
	}
}

// listSearchesCaptionAndDescription finds items by their caption or description and not by their alt text.
func listSearchesCaptionAndDescription(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	captioned := MustImage(t, "2026/08/cliff.jpg", "cliff", author)
	captioned.Caption = "A lighthouse above the bay"
	MustCreate(t, s.Media, captioned)
	described := MustImage(t, "2026/08/manual.jpg", "manual", author)
	described.Description = "Keeping the lighthouse lamp lit"
	MustCreate(t, s.Media, described)
	altered := MustImage(t, "2026/08/harbor.jpg", "harbor", author)
	altered.AltText = "A lighthouse at dusk"
	MustCreate(t, s.Media, altered)

	got := narrowedTitles(t, s.Media, media.Filter{Search: "lighthouse"})

	if want := []string{"manual", "cliff"}; !slices.Equal(got, want) {
		t.Errorf("List(lighthouse) = %q, want the caption and description matches %q", got, want)
	}
}

// listTreatsPatternCharactersAsText reads a wildcard in a search or a content type prefix as plain text.
func listTreatsPatternCharactersAsText(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))

	items, total, err := s.Media.List(t.Context(), media.Filter{Search: "h%r", Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("List(h%%r) = %d items with total %d, want the wildcard read as text", len(items), total)
	}

	wildcard, wildcardTotal, err := s.Media.List(
		t.Context(), media.Filter{Mimes: []string{"%"}, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List(mime %%) error = %v, want nil", err)
	}
	if wildcardTotal != 0 || len(wildcard) != 0 {
		t.Errorf("List(mime %%) = %d items with total %d, want the wildcard read as text",
			len(wildcard), wildcardTotal)
	}
}

// listPaginates counts every item across pages and lists the remainder on the last page.
func listPaginates(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	for _, file := range []string{"2026/08/one.jpg", "2026/08/two.jpg", "2026/08/three.jpg"} {
		MustCreate(t, s.Media, MustImage(t, file, file, author))
	}

	secondPage, total, err := s.Media.List(t.Context(), media.Filter{Page: 2, PerPage: 2})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want every item counted across pages", total)
	}
	if len(secondPage) != 1 {
		t.Errorf("second page holds %d items, want the remainder", len(secondPage))
	}
}

// listSortsTitlesAndAuthorsWithoutRegardToCase sorts titles and author names in an order a byte sort gets wrong.
func listSortsTitlesAndAuthorsWithoutRegardToCase(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	lowerCase := s.AddAuthor(t, "an author in lower case")
	day := func(month time.Month) time.Time { return time.Date(2026, month, 10, 0, 0, 0, 0, time.UTC) }
	mustCreateMediaAt(t, s.Media, "Banana", lowerCase, day(time.March), day(time.March))
	mustCreateMediaAt(t, s.Media, "cherry", author, day(time.May), day(time.May))
	mustCreateMediaAt(t, s.Media, "apricot", author, day(time.July), day(time.July))

	tests := []struct {
		orderBy media.OrderBy
		order   media.Order
		want    []string
	}{
		{media.OrderByTitle, media.OrderAsc, []string{"apricot", "Banana", "cherry"}},
		{media.OrderByTitle, media.OrderDesc, []string{"cherry", "Banana", "apricot"}},
		{media.OrderByAuthor, media.OrderAsc, []string{"Banana", "apricot", "cherry"}},
		{media.OrderByAuthor, media.OrderDesc, []string{"apricot", "cherry", "Banana"}},
	}
	for _, tt := range tests {
		got := listedTitles(t, s.Media, media.Filter{OrderBy: tt.orderBy, Order: tt.order})

		if !slices.Equal(got, tt.want) {
			t.Errorf("List(%q %q) = %q, want %q, so the sort is not case insensitive", tt.orderBy, tt.order, got, tt.want)
		}
	}
}

// listSearchesWithoutRegardToCaseBeyondASCII finds a title by a search that differs from it in the case of an accent.
func listSearchesWithoutRegardToCaseBeyondASCII(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/pier.jpg", "Café at the pier", author))
	MustCreate(t, s.Media, MustImage(t, "2026/08/cliff.jpg", "Cliff", author))

	got := narrowedTitles(t, s.Media, media.Filter{Search: "CAFÉ"})

	if want := []string{"Café at the pier"}; !slices.Equal(got, want) {
		t.Errorf("List(CAFÉ) = %q, want %q, so the search folds only ASCII letters", got, want)
	}
}
