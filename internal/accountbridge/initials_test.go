// SPDX-License-Identifier: Apache-2.0

package accountbridge_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"html"
	"html/template"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/avatar"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/sdk"
)

// initialsPrefix starts every address an initials avatar holds.
const initialsPrefix = "data:image/svg+xml;base64,"

// initialsFixture is the shared list of names with the colours and letter the admin draws for each.
type initialsFixture struct {
	Avatars []struct {
		Name         string `json:"name"`
		Circle       string `json:"circle"`
		LetterColour string `json:"letter_colour"`
		Letter       string `json:"letter"`
	} `json:"avatars"`
}

// drawnInitials is the picture an initials avatar holds: its circle colour, its letter colour and its letter.
type drawnInitials struct {
	Circle struct {
		Fill string `xml:"fill,attr"`
	} `xml:"circle"`
	Text struct {
		Fill   string `xml:"fill,attr"`
		Letter string `xml:",chardata"`
	} `xml:"text"`
}

// readInitialsFixture returns the names the shared fixture holds.
func readInitialsFixture(t *testing.T) initialsFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/initials.json")
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	var fixture initialsFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decoding the fixture: %v", err)
	}
	if len(fixture.Avatars) == 0 {
		t.Fatal("the fixture holds no names")
	}
	return fixture
}

// drawn returns the picture an initials avatar address holds.
func drawn(t *testing.T, address string) drawnInitials {
	t.Helper()
	encoded, found := strings.CutPrefix(address, initialsPrefix)
	if !found {
		t.Fatalf("address = %q, want it to start with %q", address, initialsPrefix)
	}
	svg, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decoding the picture: %v", err)
	}
	var picture drawnInitials
	if err := xml.Unmarshal(svg, &picture); err != nil {
		t.Fatalf("reading the picture %s: %v", svg, err)
	}
	return picture
}

func TestAnInitialsAvatarKeepsItsPictureInAnHTMLTemplate(t *testing.T) {
	t.Parallel()

	member := confirmed("Maria Perez", "maria@example.com", "member")
	reader := accountbridge.New(&countedUsers{held: []gouncer.User{member}}, accountbridge.Options{
		Avatar: func(name, email string, size int) string {
			return avatar.Address(name, email, size, avatar.Options{})
		},
		Timeout: time.Second,
	})
	reader.Open()
	people, err := reader.People(context.Background(), []sdk.ID{idOf(member)}, 48)
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}

	var page strings.Builder
	img := template.Must(template.New("avatar").Parse(`<img src="{{.Avatar}}">`))
	data := struct{ Avatar template.URL }{template.URL(people[idOf(member)].Avatar)}
	if err := img.Execute(&page, data); err != nil {
		t.Fatalf("rendering the avatar: %v", err)
	}

	address := people[idOf(member)].Avatar
	read := html.UnescapeString(page.String())
	if read != `<img src="`+address+`">` || !strings.HasPrefix(address, initialsPrefix) {
		t.Errorf("a browser reads %s, want the picture's address %q kept", read, address)
	}
}

// TestInitialsAvatarPaintsTheAdminColoursForEachFixtureName reads internal/accountbridge/testdata/initials.json.
func TestInitialsAvatarPaintsTheAdminColoursForEachFixtureName(t *testing.T) {
	t.Parallel()

	for _, want := range readInitialsFixture(t).Avatars {
		t.Run(want.Name, func(t *testing.T) {
			t.Parallel()

			got := drawn(t, avatar.Address(want.Name, "", 32, avatar.Options{Mode: avatar.ModeInitials}))

			painted := [3]string{got.Circle.Fill, got.Text.Fill, got.Text.Letter}
			if wanted := [3]string{want.Circle, want.LetterColour, want.Letter}; painted != wanted {
				t.Errorf("%q draws the circle, letter colour and letter %q, want %q", want.Name, painted, wanted)
			}
		})
	}
}
