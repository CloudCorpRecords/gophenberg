// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/avatar"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/sdk"
)

// namesID is the id of the stand-in plugin that shows the accounts it asks for.
const namesID = "names"

// peoplePath is the public path the names plugin answers on.
const peoplePath = "/people"

// askedSize is the avatar size the names plugin asks for unless a scenario names one.
const askedSize = 48

// drawnPrefix starts the address of an avatar the site drew itself.
const drawnPrefix = "data:image/svg+xml;base64,"

// gravatarHost is the address every Gravatar avatar starts with.
const gravatarHost = "https://secure.gravatar.com/avatar/"

// shown is one account as the names plugin shows it, named as the plugin received it.
type shown struct {
	Name     string
	Role     string
	Avatar   string
	Disabled bool
}

// namesPlugin stands in for a plugin that shows the accounts it asks the site for.
type namesPlugin struct {
	accounts sdk.AccountReader
}

// ServeHTTP reads the asked ids at the asked avatar size and shows every account found whole, keyed by id.
func (p namesPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ids, size, err := askedIn(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	people, err := p.accounts.People(r.Context(), ids, size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(people)
}

// askedIn returns the ids and the avatar size a request to the names plugin asks for.
func askedIn(query url.Values) ([]sdk.ID, int, error) {
	size, err := strconv.Atoi(query.Get("size"))
	if err != nil {
		return nil, 0, fmt.Errorf("reading the size: %w", err)
	}
	var ids []sdk.ID
	for text := range strings.SplitSeq(query.Get("ids"), ",") {
		if text == "" {
			continue
		}
		var id sdk.ID
		if err := id.UnmarshalText([]byte(text)); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	return ids, size, nil
}

// namesOver returns the names plugin over an open reader of the scenario's accounts, drawing avatars as options say.
func namesOver(w *world, options avatar.Options) namesPlugin {
	accounts := accountbridge.New(w.users, accountbridge.Options{
		Avatar: func(name, email string, size int) string {
			return avatar.Address(name, email, size, options)
		},
		Timeout: time.Second,
	})
	accounts.Open()
	return namesPlugin{accounts: accounts}
}

// theseAccounts stores each account of the table, confirmed, disabled or unconfirmed as its state says.
func theseAccounts(ctx context.Context, table *godog.Table) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		name, email, held, state := row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, row.Cells[3].Value
		account := gouncer.User{
			ID: uuid.New(), Name: name, Email: email, Role: held,
			Confirmed: state != "unconfirmed", Disabled: state == "disabled",
		}
		if err := w.users.CreateUser(ctx, account); err != nil {
			return fmt.Errorf("storing %s: %w", name, err)
		}
	}
	return nil
}

// aRunningGophenbergServingTheNamesPlugin starts a site serving a plugin that shows the accounts it asks for.
func aRunningGophenbergServingTheNamesPlugin(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.plugins = map[string]http.Handler{namesID: namesOver(w, avatar.Options{})}
	w.publicPaths = map[string][]string{namesID: {peoplePath}}
	return w.start(ctx)
}

// theSiteDrawsAvatarsFrom serves the site again with the names plugin drawing avatars in the mode.
func theSiteDrawsAvatarsFrom(ctx context.Context, mode string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	drawing, err := avatar.ParseMode(mode)
	if err != nil {
		return err
	}
	w.plugins[namesID] = namesOver(w, avatar.Options{Mode: drawing})
	return w.rebuild(ctx)
}

// peopleAsked is what one request to the names plugin asks for: the account ids and the avatar size.
type peopleAsked struct {
	ids  []uuid.UUID
	size int
}

// thePluginAsksForIDs asks the names plugin to show the accounts asked for.
func thePluginAsksForIDs(ctx context.Context, asked peopleAsked) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	texts := make([]string, 0, len(asked.ids))
	for _, id := range asked.ids {
		texts = append(texts, id.String())
	}
	query := url.Values{"ids": {strings.Join(texts, ",")}, "size": {strconv.Itoa(asked.size)}}
	if err := w.get("/api/plugins/" + namesID + peoplePath + "?" + query.Encode()); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// idsNamed returns the ids of the stored accounts named.
func idsNamed(ctx context.Context, names ...string) ([]uuid.UUID, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, found := w.users.idNamed(name)
		if !found {
			return nil, fmt.Errorf("no account is named %q", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// thePluginAsksForOne asks the names plugin to show the one stored account named.
func thePluginAsksForOne(ctx context.Context, name string) error {
	ids, err := idsNamed(ctx, name)
	if err != nil {
		return err
	}
	return thePluginAsksForIDs(ctx, peopleAsked{ids: ids, size: askedSize})
}

// thePluginAsksForOneAt asks the names plugin to show the one stored account named, with its avatar at size pixels.
func thePluginAsksForOneAt(ctx context.Context, name string, size int) error {
	ids, err := idsNamed(ctx, name)
	if err != nil {
		return err
	}
	return thePluginAsksForIDs(ctx, peopleAsked{ids: ids, size: size})
}

// thePluginAsksForTwo asks the names plugin to show the two stored accounts named.
func thePluginAsksForTwo(ctx context.Context, first, second string) error {
	ids, err := idsNamed(ctx, first, second)
	if err != nil {
		return err
	}
	return thePluginAsksForIDs(ctx, peopleAsked{ids: ids, size: askedSize})
}

// thePluginAsksForOneAndAnUnheldAccount asks the names plugin for the named account and an id no account holds.
func thePluginAsksForOneAndAnUnheldAccount(ctx context.Context, name string) error {
	ids, err := idsNamed(ctx, name)
	if err != nil {
		return err
	}
	return thePluginAsksForIDs(ctx, peopleAsked{ids: append(ids, uuid.New()), size: askedSize})
}

// thePluginAsksForNoAccounts asks the names plugin to show no account at all.
func thePluginAsksForNoAccounts(ctx context.Context) error {
	return thePluginAsksForIDs(ctx, peopleAsked{size: askedSize})
}

// shownAccounts returns the accounts the last answer shows, keyed by name.
func shownAccounts(ctx context.Context) (map[string]shown, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, err
	}
	var byID map[string]shown
	if err := w.answer.decode(&byID); err != nil {
		return nil, err
	}
	byName := make(map[string]shown, len(byID))
	for _, account := range byID {
		byName[account.Name] = account
	}
	return byName, nil
}

// shownAccount returns the named account the last answer shows.
func shownAccount(ctx context.Context, name string) (shown, error) {
	accounts, err := shownAccounts(ctx)
	if err != nil {
		return shown{}, err
	}
	account, found := accounts[name]
	if !found {
		return shown{}, fmt.Errorf("the plugin shows %+v, want %q among them", accounts, name)
	}
	return account, nil
}

// itShowsWithTheRole asserts the answer shows the named account holding the role.
func itShowsWithTheRole(ctx context.Context, name, held string) error {
	account, err := shownAccount(ctx, name)
	if err != nil {
		return err
	}
	if account.Role != held {
		return fmt.Errorf("the plugin shows %q with the role %q, want %q", name, account.Role, held)
	}
	return nil
}

// itShowsOnly asserts the answer shows the named account and no other.
func itShowsOnly(ctx context.Context, name string) error {
	accounts, err := shownAccounts(ctx)
	if err != nil {
		return err
	}
	if names := slices.Collect(maps.Keys(accounts)); !slices.Equal(names, []string{name}) {
		return fmt.Errorf("the plugin shows %v, want only %q", names, name)
	}
	return nil
}

// itShowsNobody asserts the answer shows no account.
func itShowsNobody(ctx context.Context) error {
	accounts, err := shownAccounts(ctx)
	if err != nil {
		return err
	}
	if len(accounts) != 0 {
		return fmt.Errorf("the plugin shows %+v, want nobody", accounts)
	}
	return nil
}

// itShowsAsDisabled asserts the answer shows the named account marked disabled.
func itShowsAsDisabled(ctx context.Context, name string) error {
	account, err := shownAccount(ctx, name)
	if err != nil {
		return err
	}
	if !account.Disabled {
		return fmt.Errorf("the plugin shows %+v, want %q marked disabled", account, name)
	}
	return nil
}

// theAnswerHoldsNoEmailAddress asserts the plugin shows accounts and holds no stored email, raw or escaped for a query.
func theAnswerHoldsNoEmailAddress(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	accounts, err := shownAccounts(ctx)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return fmt.Errorf("the plugin received %s, want accounts", w.answer.body)
	}
	stored, err := w.users.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range stored {
		if trace := emailTrace(w.answer.body, user.Email); trace != "" {
			return fmt.Errorf("the plugin received %s, want no trace of %q, found %q", w.answer.body, user.Email, trace)
		}
	}
	return nil
}

// emailTrace returns the form of email that body holds, raw or escaped for a query, empty when it holds neither.
func emailTrace(body []byte, email string) string {
	for _, form := range []string{email, url.QueryEscape(email)} {
		if bytes.Contains(body, []byte(form)) {
			return form
		}
	}
	return ""
}

// theAvatarIsAPictureTheSiteDrew asserts the named account's avatar is a picture inside the address showing the letter.
func theAvatarIsAPictureTheSiteDrew(ctx context.Context, name, letter string) error {
	account, err := shownAccount(ctx, name)
	if err != nil {
		return err
	}
	encoded, inside := strings.CutPrefix(account.Avatar, drawnPrefix)
	if !inside {
		return fmt.Errorf("the avatar of %q is %q, want a picture inside the address", name, account.Avatar)
	}
	picture, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decoding the avatar of %q: %w", name, err)
	}
	if !bytes.HasPrefix(picture, []byte("<svg")) || !bytes.Contains(picture, []byte(">"+letter+"</text>")) {
		return fmt.Errorf("the avatar of %q draws %s, want a picture with the letter %q", name, picture, letter)
	}
	return nil
}

// theAvatarIsTheGravatarOf asserts the named account's avatar is the Gravatar of the email's hash at size pixels.
func theAvatarIsTheGravatarOf(ctx context.Context, name, email string, size int) error {
	account, err := shownAccount(ctx, name)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	address, err := url.Parse(account.Avatar)
	if err != nil {
		return fmt.Errorf("reading the avatar of %q: %w", name, err)
	}
	asked := address.Query().Get("s")
	address.RawQuery = ""
	if address.String() != gravatarHost+hex.EncodeToString(hash[:]) || asked != strconv.Itoa(size) {
		return fmt.Errorf("the avatar of %q is %q, want the Gravatar of the hash of %q at %d pixels",
			name, account.Avatar, email, size)
	}
	return nil
}

// theAvatarIsEmpty asserts the named account carries no avatar.
func theAvatarIsEmpty(ctx context.Context, name string) error {
	account, err := shownAccount(ctx, name)
	if err != nil {
		return err
	}
	if account.Avatar != "" {
		return fmt.Errorf("the avatar of %q is %q, want it empty", name, account.Avatar)
	}
	return nil
}

// theSiteReadTheAccounts asserts how many reads of many ids the site made.
func theSiteReadTheAccounts(ctx context.Context, want int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if got := w.users.readsByIDs(); got != want {
		return fmt.Errorf("the site read the accounts %d times, want %d", got, want)
	}
	return nil
}

// theSiteReadTheAccountsOnce asserts the site read the accounts in one call.
func theSiteReadTheAccountsOnce(ctx context.Context) error {
	return theSiteReadTheAccounts(ctx, 1)
}

// theSiteReadNoAccounts asserts the site never read the accounts.
func theSiteReadNoAccounts(ctx context.Context) error {
	return theSiteReadTheAccounts(ctx, 0)
}

// initializePluginAccounts binds the steps the plugin accounts feature uses.
func initializePluginAccounts(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^these accounts$`, theseAccounts)
	sc.Given(
		`^a running Gophenberg serving the plugin "`+namesID+`", which shows the accounts it asks for$`,
		aRunningGophenbergServingTheNamesPlugin,
	)
	sc.Given(`^the site draws avatars from "([^"]*)"$`, theSiteDrawsAvatarsFrom)
	sc.When(`^the plugin "`+namesID+`" asks for "([^"]*)"$`, thePluginAsksForOne)
	sc.When(`^the plugin "`+namesID+`" asks for "([^"]*)" at (\d+) pixels$`, thePluginAsksForOneAt)
	sc.When(`^the plugin "`+namesID+`" asks for "([^"]*)" and "([^"]*)"$`, thePluginAsksForTwo)
	sc.When(
		`^the plugin "`+namesID+`" asks for "([^"]*)" and an account the site does not hold$`,
		thePluginAsksForOneAndAnUnheldAccount,
	)
	sc.When(`^the plugin "`+namesID+`" asks for no accounts$`, thePluginAsksForNoAccounts)
	sc.Then(`^it shows "([^"]*)" with the role "([^"]*)"$`, itShowsWithTheRole)
	sc.Then(`^it shows only "([^"]*)"$`, itShowsOnly)
	sc.Then(`^it shows nobody$`, itShowsNobody)
	sc.Then(`^it shows "([^"]*)" as disabled$`, itShowsAsDisabled)
	sc.Then(`^the answer holds no email address$`, theAnswerHoldsNoEmailAddress)
	sc.Then(`^the avatar of "([^"]*)" is a picture the site drew with the letter "([^"]*)"$`,
		theAvatarIsAPictureTheSiteDrew)
	sc.Then(`^the avatar of "([^"]*)" is the Gravatar of "([^"]*)" at (\d+) pixels$`, theAvatarIsTheGravatarOf)
	sc.Then(`^the avatar of "([^"]*)" is empty$`, theAvatarIsEmpty)
	sc.Then(`^the site read the accounts once$`, theSiteReadTheAccountsOnce)
	sc.Then(`^the site read no accounts$`, theSiteReadNoAccounts)
}
