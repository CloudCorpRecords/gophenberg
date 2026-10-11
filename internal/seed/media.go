// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/media"
)

// demoMediaQuality is the JPEG quality the scripted pictures encode at.
const demoMediaQuality = 82

// seedListSize bounds the listing one title probe reads.
const seedListSize = 100

// MediaLibrary stores an upload's files, marks them finished and takes them back.
type MediaLibrary interface {
	Ingest(ctx context.Context, name string, data []byte, authorID uuid.UUID) (media.Media, error)
	Finish(m media.Media)
	Remove(m media.Media) error
}

// demoUpload is one scripted upload of the demo data set, a picture or a document.
type demoUpload struct {
	file        string
	title       string
	altText     string
	caption     string
	description string
	width       int
	height      int
	paint       func(u demoUpload, x, y int) color.RGBA
	author      string
	daysAgo     int
	editedAfter int
}

// document reports whether the upload is a document rather than a picture.
func (u demoUpload) document() bool {
	return strings.HasSuffix(u.file, ".pdf")
}

// data returns the file the upload sends, a one-page PDF for a document and a painted JPEG for a picture.
func (u demoUpload) data() ([]byte, error) {
	if u.document() {
		return demoPDF(u.title), nil
	}
	return u.build()
}

// demoPDF returns a one-page PDF printing the title, its cross reference table pointing at every object.
func demoPDF(title string) []byte {
	text := fmt.Sprintf("BT /F1 24 Tf 72 720 Td (%s) Tj ET", title)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(text), text),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	table := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, table)
	return out.Bytes()
}

// build returns the JPEG bytes of the scripted picture.
func (u demoUpload) build() ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, u.width, u.height))
	for y := range u.height {
		for x := range u.width {
			canvas.Set(x, y, u.paint(u, x, y))
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: demoMediaQuality}); err != nil {
		return nil, fmt.Errorf("paint %q: %w", u.file, err)
	}
	return buffer.Bytes(), nil
}

// demoMedia returns the scripted uploads stored by [Media], eleven pictures and a document over eighteen months.
func demoMedia() []demoUpload {
	return []demoUpload{
		{
			file: "harbor-at-dawn.jpg", title: "Harbor at dawn",
			altText:     "Bands of orange and blue over a flat horizon",
			caption:     "The harbor before the market opens",
			description: "A wide view of the water taken from the eastern pier.",
			width:       1600, height: 1000, paint: dawnPixel, author: AdminEmail, daysAgo: 3,
		},
		{
			file: "mountain-ridge.jpg", title: "Mountain ridge",
			altText:     "A dark ridge line against a pale sky",
			caption:     "The ridge seen from the valley floor",
			description: "The last light of the day along the northern ridge.",
			width:       1200, height: 800, paint: ridgePixel, author: EditorEmail, daysAgo: 45, editedAfter: 30,
		},
		{
			file: "visitor-guide.pdf", title: "Visitor guide",
			caption:     "Opening hours and the map of the grounds",
			description: "The printed guide handed out at the front desk.",
			author:      AdminEmail, daysAgo: 60,
		},
		{
			file: "city-lights.jpg", title: "City lights",
			altText:     "Scattered points of light across a dark grid",
			caption:     "The quarter after the offices empty",
			description: "Windows still lit in the blocks around the old exchange.",
			width:       900, height: 1200, paint: cityPixel, author: AuthorEmail, daysAgo: 96,
		},
		{
			file: "morning-tide.jpg", title: "Morning tide",
			altText:     "Calm water under a warm sky",
			caption:     "The tide coming in before the first boats",
			description: "The bay at low light, an hour after sunrise.",
			width:       1400, height: 900, paint: dawnPixel, author: AuthorEmail, daysAgo: 150,
		},
		{
			file: "northern-pass.jpg", title: "Northern pass",
			altText:     "A narrow gap between two dark slopes",
			caption:     "The pass the old road climbs through",
			description: "The road over the northern pass, closed in winter.",
			width:       1000, height: 700, paint: ridgePixel, author: AdminEmail, daysAgo: 212,
		},
		{
			file: "station-at-night.jpg", title: "Station at night",
			altText:     "Rows of lit windows in a long dark building",
			caption:     "The station after the last train",
			description: "The main hall seen from the square at closing time.",
			width:       800, height: 1000, paint: cityPixel, author: EditorEmail, daysAgo: 268,
		},
		{
			file: "fishing-boats.jpg", title: "Fishing boats",
			altText:     "Small boats on still water at sunrise",
			caption:     "The fleet back from the night catch",
			description: "Boats tied along the quay while the crews unload.",
			width:       1280, height: 720, paint: dawnPixel, author: EditorEmail, daysAgo: 321,
		},
		{
			file: "snow-line.jpg", title: "Snow line",
			altText:     "A pale band above a dark slope",
			caption:     "Where the first snow stopped this year",
			description: "The snow line seen from the lower meadow.",
			width:       1100, height: 750, paint: ridgePixel, author: AuthorEmail, daysAgo: 377,
		},
		{
			file: "old-exchange.jpg", title: "Old exchange",
			altText:     "A grid of windows with a few lit",
			caption:     "The old exchange before the renovation",
			description: "The trading hall, now offices, from across the street.",
			width:       720, height: 960, paint: cityPixel, author: AdminEmail, daysAgo: 430,
		},
		{
			file: "pier-in-the-fog.jpg", title: "Pier in the fog",
			altText:     "Soft light over grey water",
			caption:     "The pier on a still morning",
			description: "The end of the pier lost in the morning fog.",
			width:       960, height: 640, paint: dawnPixel, author: AuthorEmail, daysAgo: 488,
		},
		{
			file: "valley-floor.jpg", title: "Valley floor",
			altText:     "A ridge line over a wide dark valley",
			caption:     "The valley from the last bend of the road",
			description: "Fields along the river below the southern ridge.",
			width:       1500, height: 1000, paint: ridgePixel, author: EditorEmail, daysAgo: 534,
		},
	}
}

// dawnPixel paints a sunrise gradient over water.
func dawnPixel(u demoUpload, x, y int) color.RGBA {
	sky := float64(y) / float64(u.height)
	if sky < 0.6 {
		warm := 1 - sky/0.6
		return color.RGBA{R: uint8(90 + 150*warm), G: uint8(110 + 90*warm), B: uint8(160 - 40*warm), A: 255}
	}
	ripple := 0.5 + 0.5*math.Sin(float64(x)/40+float64(y)/12)
	return color.RGBA{R: uint8(20 + 40*ripple), G: uint8(50 + 50*ripple), B: uint8(90 + 60*ripple), A: 255}
}

// ridgePixel paints a mountain ridge under a pale sky.
func ridgePixel(u demoUpload, x, y int) color.RGBA {
	crest := float64(u.height)*0.5 + 120*math.Sin(float64(x)/220) + 60*math.Sin(float64(x)/70)
	if float64(y) < crest {
		pale := float64(y) / crest
		return color.RGBA{R: uint8(210 - 40*pale), G: uint8(220 - 30*pale), B: uint8(235 - 20*pale), A: 255}
	}
	depth := (float64(y) - crest) / float64(u.height)
	return color.RGBA{R: uint8(60 - 30*depth), G: uint8(70 - 30*depth), B: uint8(80 - 30*depth), A: 255}
}

// cityPixel paints lit windows across a dark grid.
func cityPixel(_ demoUpload, x, y int) color.RGBA {
	lit := (x/24+y/18)%7 == 0 && x%24 < 14 && y%18 < 10
	if lit {
		return color.RGBA{R: 240, G: 220, B: 150, A: 255}
	}
	glow := 0.5 + 0.5*math.Sin(float64(y)/300)
	return color.RGBA{R: uint8(16 + 12*glow), G: uint8(18 + 14*glow), B: uint8(32 + 20*glow), A: 255}
}

// Media stores the scripted uploads the library does not already hold, each under its demo account.
func Media(ctx context.Context, library MediaLibrary, store media.Store, users gouncer.Store) error {
	writers, err := demoWriters(ctx, users)
	if err != nil {
		return err
	}
	for _, scripted := range demoMedia() {
		held, err := libraryHolds(ctx, store, scripted.title)
		if err != nil {
			return err
		}
		if held {
			continue
		}
		if err := storeDemoUpload(ctx, library, store, scripted, writers[scripted.author]); err != nil {
			return err
		}
	}
	return nil
}

// libraryHolds reports whether the library already holds an item titled title.
func libraryHolds(ctx context.Context, store media.Store, title string) (bool, error) {
	stored, _, err := store.List(ctx, media.Filter{Search: title, Page: 1, PerPage: seedListSize})
	if err != nil {
		return false, fmt.Errorf("seed media lookup: %w", err)
	}
	for _, item := range stored {
		if item.Title == title {
			return true, nil
		}
	}
	return false, nil
}

// storeDemoUpload builds one scripted upload and stores it with its descriptions, dated as the script says.
func storeDemoUpload(
	ctx context.Context, library MediaLibrary, store media.Store, scripted demoUpload, authorID uuid.UUID,
) error {
	data, err := scripted.data()
	if err != nil {
		return err
	}
	item, err := library.Ingest(ctx, scripted.file, data, authorID)
	if err != nil {
		return fmt.Errorf("seed media ingest: %w", err)
	}
	item.Title = scripted.title
	item.AltText = scripted.altText
	item.Caption = scripted.caption
	item.Description = scripted.description
	datedUpload(&item, scripted)
	created, err := store.Create(ctx, item)
	if err != nil {
		return errors.Join(fmt.Errorf("seed media: %w", err), library.Remove(item))
	}
	library.Finish(created)
	return nil
}

// datedUpload moves the item back by the days its script gives, last modified the days after that it was edited.
func datedUpload(item *media.Media, scripted demoUpload) {
	added := item.CreatedAt.AddDate(0, 0, -scripted.daysAgo)
	item.CreatedAt, item.UpdatedAt = added, added.AddDate(0, 0, scripted.editedAfter)
}
