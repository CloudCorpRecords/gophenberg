// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// choosing returns settings in which the site chose the named theme.
func choosing(name string) *fakeSettings {
	settings := newSettings()
	settings.values[themehost.ActiveKey] = name
	return settings
}

// slowToServe tunes a supervisor to keep waiting for a theme that never answers its probe, and to stop it quickly.
func slowToServe(supervision *themehost.SupervisorConfig) {
	supervision.ReadyTimeout = 6 * time.Second
	supervision.StopGrace = 250 * time.Millisecond
}

func TestCheckChoiceWaitsForTheThemeTheSiteChoseToServe(t *testing.T) {
	t.Parallel()

	for name, choice := range map[string]struct{ stored, pinned string }{
		"the stored choice": {stored: "aurora"},
		"an operator pin":   {pinned: "aurora"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			manager, logs := managerServing(t, choosing(choice.stored), choice.pinned)
			if err := manager.Boot(t.Context()); err != nil {
				t.Fatalf("Boot() = %v, want nil", err)
			}

			err := manager.CheckChoice(t.Context())

			if err != nil {
				t.Fatalf("CheckChoice() = %v, want nil\nsupervisor logs:\n%s", err, logs)
			}
			if !manager.Holder().Healthy() {
				t.Error("CheckChoice() passed while the built-in renderer still answered in place of the chosen theme")
			}
		})
	}
}

func TestCheckChoicePassesASiteThatChoseNoThemeAtOnce(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "aurora")
	if err := manager.Boot(t.Context()); err != nil {
		t.Fatalf("Boot() = %v, want nil", err)
	}
	ended, end := context.WithCancel(t.Context())
	end()

	if err := manager.CheckChoice(ended); err != nil {
		t.Errorf("CheckChoice() = %v, want a site that chose the built-in renderer passed without waiting", err)
	}
}

func TestCheckChoiceRefusesASiteTheRendererServesInPlaceOfTheThemeItChose(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, choosing("vanished"), "")
	if err := manager.Boot(t.Context()); err != nil {
		t.Fatalf("Boot() = %v, want the server to stay up", err)
	}

	err := manager.CheckChoice(t.Context())

	if err == nil || err.Error() != "themehost: the site chose the theme vanished, which does not serve the public site" {
		t.Errorf("CheckChoice() = %v, want the theme the site chose named", err)
	}
}

func TestCheckChoiceRefusesASiteServedThroughAThemeOtherThanTheOneItChose(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	manager, logs := managerServing(t, settings, "")
	if err := manager.Activate(t.Context(), "aurora"); err != nil {
		t.Fatalf("Activate() = %v, want aurora serving\nsupervisor logs:\n%s", err, logs)
	}
	settings.values[themehost.ActiveKey] = "riverbed"

	err := manager.CheckChoice(t.Context())

	if err == nil || err.Error() != "themehost: the site chose the theme riverbed, which does not serve the public site" {
		t.Errorf("CheckChoice() = %v, want the theme the site chose named", err)
	}
}

func TestCheckChoiceRefusesAChosenThemeThatGaveUpStarting(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, choosing("aurora"), "", "aurora")
	if err := manager.Boot(t.Context()); err != nil {
		t.Fatalf("Boot() = %v, want the server up while the chosen theme fails to start", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	err := manager.CheckChoice(ctx)

	if !errors.Is(err, themehost.ErrStartFailed) {
		t.Fatalf("CheckChoice() = %v, want the theme that stopped trying to start refused", err)
	}
	want := "themehost: the site chose the theme aurora, which did not start serving the public site: " +
		themehost.ErrStartFailed.Error()
	if err.Error() != want {
		t.Errorf("CheckChoice() = %q, want %q", err, want)
	}
}

func TestCheckChoiceRefusesAChosenThemeStillStartingWhenTheCheckEnds(t *testing.T) {
	t.Parallel()

	manager := managerRunningOver(t, choosing("aurora"), "deaf", slowToServe)
	if err := manager.Boot(t.Context()); err != nil {
		t.Fatalf("Boot() = %v, want the server up while the chosen theme starts", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := manager.CheckChoice(ctx)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CheckChoice() = %v, want the theme that never served before the check ended refused", err)
	}
	want := "themehost: the site chose the theme aurora, which did not start serving the public site: " +
		context.DeadlineExceeded.Error()
	if err.Error() != want {
		t.Errorf("CheckChoice() = %q, want %q", err, want)
	}
}

// askedSettings holds theme choices and tells asked each time one is looked up.
type askedSettings struct {
	fakeSettings
	asked chan struct{}
}

// Lookup tells asked, then returns the value stored under key and whether the key is set at all.
func (a *askedSettings) Lookup(ctx context.Context, key string) (string, bool, error) {
	a.asked <- struct{}{}
	return a.fakeSettings.Lookup(ctx, key)
}

func TestCloseInterruptsACheckStillWaitingForTheChosenTheme(t *testing.T) {
	t.Parallel()

	settings := &askedSettings{asked: make(chan struct{}, 1)}
	settings.values = map[string]string{themehost.ActiveKey: "aurora"}
	manager := managerRunningOver(t, settings, "deaf", slowToServe)
	if err := manager.Boot(t.Context()); err != nil {
		t.Fatalf("Boot() = %v, want the server up while the chosen theme starts", err)
	}
	<-settings.asked
	checked := make(chan error, 1)
	go func() { checked <- manager.CheckChoice(context.Background()) }()
	<-settings.asked

	began := time.Now()
	manager.Close()
	took := time.Since(began)

	if took > 2*time.Second {
		t.Errorf("Close() took %v, want the waiting check interrupted well inside a stop grace", took)
	}
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Errorf("CheckChoice() = %v, want the check the manager closed under refused", err)
	}
}
