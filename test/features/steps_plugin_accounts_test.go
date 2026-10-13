// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
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

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/sdk"
)

// peoplePath is the public path the names plugin answers on.
const peoplePath = "/people"

// askedSize is the avatar size the names plugin asks for unless a scenario names one.
const askedSize = 48

// shown is one account as the names plugin shows it.
type shown struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Avatar   string `json:"avatar"`
	Disabled bool   `json:"disabled"`
}

// namesPlugin stands in for a plugin that shows the accounts it asks the site for.
type namesPlugin struct {
	accounts sdk.AccountReader
}

// ServeHTTP reads the asked ids at the asked avatar size and shows each account found, keyed by id.
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
	shownByID := make(map[string]shown, len(people))
	for id, person := range people {
		shownByID[id.String()] = shown{Name: person.Name, Role: person.Role, Avatar: person.Avatar, Disabled: person.Disabled}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(shownByID)
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

// noAvatar draws no avatar, the site's choice until a scenario names one.
func noAvatar(string, string, int) string {
	return ""
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
func aRunningGophenbergServingTheNamesPlugin(ctx context.Context, id string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	accounts := accountbridge.New(w.users, accountbridge.Options{Avatar: noAvatar, Timeout: time.Second})
	accounts.Open()
	w.plugins = map[string]http.Handler{id: namesPlugin{accounts: accounts}}
	w.publicPaths = map[string][]string{id: {peoplePath}}
	return w.start(ctx)
}

// thePluginAsksForIDs asks the plugin to show the accounts holding ids.
func thePluginAsksForIDs(ctx context.Context, plugin string, ids []uuid.UUID) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	texts := make([]string, 0, len(ids))
	for _, id := range ids {
		texts = append(texts, id.String())
	}
	query := url.Values{"ids": {strings.Join(texts, ",")}, "size": {strconv.Itoa(askedSize)}}
	if err := w.get("/api/plugins/" + plugin + peoplePath + "?" + query.Encode()); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePluginAsksFor asks the plugin to show the stored accounts named.
func thePluginAsksFor(ctx context.Context, plugin string, names ...string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, found := w.users.idNamed(name)
		if !found {
			return fmt.Errorf("no account is named %q", name)
		}
		ids = append(ids, id)
	}
	return thePluginAsksForIDs(ctx, plugin, ids)
}

// thePluginAsksForOne asks the plugin to show the one stored account named.
func thePluginAsksForOne(ctx context.Context, plugin, name string) error {
	return thePluginAsksFor(ctx, plugin, name)
}

// thePluginAsksForTwo asks the plugin to show the two stored accounts named.
func thePluginAsksForTwo(ctx context.Context, plugin, first, second string) error {
	return thePluginAsksFor(ctx, plugin, first, second)
}

// thePluginAsksForOneAndAnUnheldAccount asks for the named account and an id no account holds.
func thePluginAsksForOneAndAnUnheldAccount(ctx context.Context, plugin, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	id, found := w.users.idNamed(name)
	if !found {
		return fmt.Errorf("no account is named %q", name)
	}
	return thePluginAsksForIDs(ctx, plugin, []uuid.UUID{id, uuid.New()})
}

// thePluginAsksForNoAccounts asks the plugin to show no account at all.
func thePluginAsksForNoAccounts(ctx context.Context, plugin string) error {
	return thePluginAsksForIDs(ctx, plugin, nil)
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

// itShowsWithTheRole asserts the answer shows the named account holding the role.
func itShowsWithTheRole(ctx context.Context, name, held string) error {
	accounts, err := shownAccounts(ctx)
	if err != nil {
		return err
	}
	if got, found := accounts[name]; !found || got.Role != held {
		return fmt.Errorf("the plugin shows %+v, want %q with the role %q", accounts, name, held)
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
	accounts, err := shownAccounts(ctx)
	if err != nil {
		return err
	}
	if got, found := accounts[name]; !found || !got.Disabled {
		return fmt.Errorf("the plugin shows %+v, want %q marked disabled", accounts, name)
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
		`^a running Gophenberg serving the plugin "([^"]*)", which shows the accounts it asks for$`,
		aRunningGophenbergServingTheNamesPlugin,
	)
	sc.When(`^the plugin "([^"]*)" asks for "([^"]*)"$`, thePluginAsksForOne)
	sc.When(`^the plugin "([^"]*)" asks for "([^"]*)" and "([^"]*)"$`, thePluginAsksForTwo)
	sc.When(
		`^the plugin "([^"]*)" asks for "([^"]*)" and an account the site does not hold$`,
		thePluginAsksForOneAndAnUnheldAccount,
	)
	sc.When(`^the plugin "([^"]*)" asks for no accounts$`, thePluginAsksForNoAccounts)
	sc.Then(`^it shows "([^"]*)" with the role "([^"]*)"$`, itShowsWithTheRole)
	sc.Then(`^it shows only "([^"]*)"$`, itShowsOnly)
	sc.Then(`^it shows nobody$`, itShowsNobody)
	sc.Then(`^it shows "([^"]*)" as disabled$`, itShowsAsDisabled)
	sc.Then(`^the site read the accounts once$`, theSiteReadTheAccountsOnce)
	sc.Then(`^the site read no accounts$`, theSiteReadNoAccounts)
}
