// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/testdb"
)

func TestSuperuserPolicyIsWarnOrRefuse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value string
		want  superuserPolicy
		err   string
	}{
		{value: "", want: superuserWarn},
		{value: "warn", want: superuserWarn},
		{value: "refuse", want: superuserRefuse},
		{value: "Refuse", err: `GOPHENBERG_SUPERUSER_POLICY: must be warn or refuse, got "Refuse"`},
		{value: "allow", err: `GOPHENBERG_SUPERUSER_POLICY: must be warn or refuse, got "allow"`},
	}
	for _, tc := range cases {
		env := settingsEnv(testkit.Getenv(map[string]string{"GOPHENBERG_SUPERUSER_POLICY": tc.value}))
		got, err := superuserPolicyFrom(env)
		switch {
		case tc.err == "" && (err != nil || got != tc.want):
			t.Errorf("superuserPolicyFrom(%q) = %q, %v, want %q", tc.value, got, err, tc.want)
		case tc.err != "" && (err == nil || err.Error() != tc.err):
			t.Errorf("superuserPolicyFrom(%q) error = %v, want %q", tc.value, err, tc.err)
		}
	}
}

// askedRole returns a question about the connected role that answers super and err.
func askedRole(super bool, err error) func(context.Context) (bool, error) {
	return func(context.Context) (bool, error) {
		return super, err
	}
}

func TestSuperuserCheckWarnsUnderWarn(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer

	err := superuserCheck(t.Context(), superuserWarn, askedRole(true, nil), slog.New(slog.NewTextHandler(&logged, nil)))

	if err != nil {
		t.Fatalf("superuserCheck() = %v under warn, want nil", err)
	}
	if !strings.Contains(logged.String(), "level=WARN") || !strings.Contains(logged.String(), superuserWarning) {
		t.Errorf("logged %q, want the superuser warning", logged.String())
	}
}

func TestSuperuserCheckRefusesUnderRefuse(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer

	err := superuserCheck(t.Context(), superuserRefuse, askedRole(true, nil), slog.New(slog.NewTextHandler(&logged, nil)))

	want := "gophenberg connects to its database as a superuser and GOPHENBERG_SUPERUSER_POLICY is refuse, " +
		"so connect as the database's own owner role"
	if err == nil || err.Error() != want {
		t.Errorf("superuserCheck() = %v under refuse, want %q", err, want)
	}
	if logged.Len() != 0 {
		t.Errorf("logged %q under refuse, want the refusal alone", logged.String())
	}
}

func TestSuperuserCheckSaysNothingForAnOwnerRole(t *testing.T) {
	t.Parallel()

	for _, policy := range []superuserPolicy{superuserWarn, superuserRefuse} {
		var logged bytes.Buffer
		err := superuserCheck(t.Context(), policy, askedRole(false, nil), slog.New(slog.NewTextHandler(&logged, nil)))
		if err != nil || logged.Len() != 0 {
			t.Errorf("superuserCheck() under %s = %v and logged %q, want nil and nothing", policy, err, logged.String())
		}
	}
}

func TestSuperuserCheckReportsAQuestionItCouldNotAsk(t *testing.T) {
	t.Parallel()

	err := superuserCheck(t.Context(), superuserWarn, askedRole(false, errRead), slog.New(slog.DiscardHandler))

	if !errors.Is(err, errRead) {
		t.Errorf("superuserCheck() = %v, want the failure of the question", err)
	}
}

// listenRecorder records what the server logs and cancels its context once the server logs that it is listening.
type listenRecorder struct {
	cancel context.CancelFunc
	logged *strings.Builder
}

// Write records p and cancels on the listening line.
func (w listenRecorder) Write(p []byte) (int, error) {
	w.logged.Write(p)
	if strings.Contains(string(p), "listening") {
		w.cancel()
	}
	return len(p), nil
}

func TestRunWarnsWhenItConnectsAsASuperuser(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": testdb.AsServer(t, emptyDatabaseURL(t)), "GOPHENBERG_ADDR": "localhost:0",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logged := &strings.Builder{}

	if err := run(ctx, testkit.Getenv(env), listenRecorder{cancel: cancel, logged: logged}, noPlugins); err != nil {
		t.Fatalf("run() error = %v as a superuser under warn, want a clean shutdown", err)
	}
	if !strings.Contains(logged.String(), superuserWarning) {
		t.Errorf("logged %q, want the superuser warning", logged.String())
	}
}

func TestRunStopsWhenItConnectsAsASuperuserUnderRefuse(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": testdb.AsServer(t, databaseURL), "GOPHENBERG_ADDR": "localhost:0",
		"GOPHENBERG_SUPERUSER_POLICY": "refuse",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, noPlugins)

	if err == nil || !strings.HasPrefix(err.Error(), "gophenberg connects to its database as a superuser") {
		t.Fatalf("run() error = %v, want the superuser refusal before the site listens", err)
	}
	if got := storedMinimumVersionAt(t, databaseURL); got != "" {
		t.Errorf("stored minimum version = %q after the refusal, want nothing migrated", got)
	}
}

func TestRunSaysNothingOfTheRoleOfADatabaseOwner(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t), "GOPHENBERG_ADDR": "localhost:0"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logged := &strings.Builder{}

	if err := run(ctx, testkit.Getenv(env), listenRecorder{cancel: cancel, logged: logged}, noPlugins); err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	if strings.Contains(logged.String(), "superuser") {
		t.Errorf("logged %q for the database's owner role, want no superuser warning", logged.String())
	}
}
