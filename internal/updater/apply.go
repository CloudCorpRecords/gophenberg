// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/version"
)

// serverLookup reads the PostgreSQL major of the server and the size in bytes of the site database.
const serverLookup = "SELECT current_setting('server_version_num')::int / 10000, pg_database_size(current_database())"

// stoppedSteps are the steps a job that stopped before its switch can have written last.
var stoppedSteps = []Step{StepDownload, StepStage, StepBackup}

// Job is one update of a server install as update:apply runs it, with what a person asked of it.
type Job struct {
	Settings    Settings
	Check       update.Config
	Store       update.Settings
	DB          *sql.DB
	DatabaseURL string
	Stamp       string
	Kit         string
	ThemeError  error
	To          string
	Retry       bool
	SkipBackup  bool
}

// server is what a job reads of the database server: its PostgreSQL major and the size of the site database.
type server struct {
	major int
	size  int64
}

// run is one job under way: the job, its install root, the state found there, the update chosen and what it read.
type run struct {
	job      Job
	layout   Layout
	held     State
	previous string
	chosen   update.Chosen
	artifact update.Artifact
	server   server
	archive  *os.File
}

// step is one step of a job that changes the install root, written to the state file before it runs.
type step struct {
	name Step
	run  func(context.Context, *State) error
}

// retryError refuses a version that failed its start check after an earlier update, naming how to retry it.
type retryError struct {
	update.RefusedError
}

// Error says the version failed before and the command that takes it again.
func (e retryError) Error() string {
	return "updater: Gophenberg " + e.Version + " failed its start check after an earlier update, so update:apply " +
		"never takes it again on its own, run update:apply -to " + e.Version + " -retry to try it again"
}

// Unwrap returns the error the check refused the version with.
func (e retryError) Unwrap() error {
	return e.RefusedError
}

// Apply locks, checks, plans, downloads, stages, backs up and switches, returning the state the job ends at.
func (j Job) Apply(ctx context.Context) (State, error) {
	facts, err := serverOf(ctx, j.DB)
	if err != nil {
		return State{}, err
	}
	release, err := Lock(ctx, j.DB)
	if err != nil {
		return State{}, err
	}
	defer release()
	r, err := j.planned(ctx, facts)
	if err != nil {
		return State{}, err
	}
	return r.apply(ctx)
}

// serverOf reads the PostgreSQL major of the server db connects to and the size of its database.
func serverOf(ctx context.Context, db *sql.DB) (server, error) {
	var facts server
	if err := db.QueryRowContext(ctx, serverLookup).Scan(&facts.major, &facts.size); err != nil {
		return server{}, fmt.Errorf("updater: read the database server: %w", err)
	}
	return facts, nil
}

// planned reads the state, chooses the update and returns the run once no check of the plan refuses it.
func (j Job) planned(ctx context.Context, facts server) (*run, error) {
	r := &run{job: j, layout: Layout{Root: j.Settings.Root}, server: facts}
	if err := r.readState(); err != nil {
		return nil, err
	}
	chosen, err := update.Choose(ctx, r.checkConfig(), j.Store, j.To)
	if err != nil {
		return nil, retryHint(err)
	}
	r.chosen = chosen
	return r, r.plan(ctx)
}

// readState reads the state the install root holds, none for an install with no install root.
func (r *run) readState() error {
	if r.layout.Root == "" {
		return nil
	}
	held, err := r.layout.ReadState()
	r.held = held
	return err
}

// checkConfig returns the check of the update site for this server, refusing the versions the state refuses.
func (r *run) checkConfig() update.Config {
	cfg := r.job.Check
	cfg.Site.ServerMajor = r.server.major
	cfg.Site.Refused = r.held.Refused
	if r.job.Retry {
		cfg.Site = cfg.Site.Lift(r.job.To)
	}
	return cfg
}

// retryHint returns err naming the command that retries the version it refused, when it refused one that failed.
func retryHint(err error) error {
	var refused update.RefusedError
	if errors.As(err, &refused) {
		return retryError{RefusedError: refused}
	}
	return err
}

// running returns the version of the binary that runs the job.
func (r *run) running() string {
	return r.job.Check.Site.Running
}

// to returns the version the job updates to.
func (r *run) to() string {
	return r.chosen.Found.Version
}

// backsUp reports whether the job takes a backup: always for a long jump, otherwise unless a person skipped it.
func (r *run) backsUp() bool {
	return r.chosen.Found.LongJump || !r.job.SkipBackup
}

// apply clears what an earlier job left when it stopped, then runs each step, putting the install root back on failure.
func (r *run) apply(ctx context.Context) (State, error) {
	r.clearInterrupted()
	r.previous, _ = r.layout.Named(Previous)
	state := State{From: r.running(), To: r.to(), LongJump: r.chosen.Found.LongJump}
	for _, s := range r.steps() {
		state.Step, state.Refused = s.name, r.refusedAt(s.name)
		err := r.layout.WriteState(state)
		if err == nil {
			err = s.run(ctx, &state)
		}
		if err != nil {
			return State{}, r.putBack(err)
		}
	}
	return state, nil
}

// steps returns the steps of the job in order, the backup only when the job takes one.
func (r *run) steps() []step {
	steps := []step{{name: StepDownload, run: r.download}, {name: StepStage, run: r.stage}}
	if r.backsUp() {
		steps = append(steps, step{name: StepBackup, run: r.backUp})
	}
	return append(steps, step{name: StepSwitched, run: r.switchBuilds})
}

// refusedAt returns the versions the state refuses at step, still the one the job takes until the switch.
func (r *run) refusedAt(step Step) []string {
	if step != StepSwitched {
		return r.held.Refused
	}
	return lifting(r.held.Refused, r.to())
}

// download fetches the archive into the downloads folder and checks it, keeping it open for the stage.
func (r *run) download(ctx context.Context, _ *State) error {
	partial := r.layout.Download(r.to())
	_ = os.MkdirAll(filepath.Dir(partial), 0o755)
	archive, err := Download{
		Artifact: r.artifact, Signers: r.chosen.Signers, Partial: partial, Cap: r.job.Settings.DownloadCap,
		Client: NewClient(r.job.Settings.DownloadTimeout),
	}.Fetch(ctx)
	r.archive = archive
	return err
}

// stage unpacks the archive into a new build folder and checks the build, deleting the download either way.
func (r *run) stage(ctx context.Context, _ *State) error {
	defer r.dropArchive()
	build := r.layout.Build(r.to())
	_ = os.MkdirAll(filepath.Dir(build), 0o755)
	_ = os.RemoveAll(build)
	if err := Unpack(r.archive, build); err != nil {
		return err
	}
	return checkBuild(ctx, r.layout.Binary(r.to()), r.to())
}

// dropArchive closes the downloaded archive and deletes it.
func (r *run) dropArchive() {
	_ = r.archive.Close()
	_ = os.Remove(r.archive.Name())
}

// checkBuild runs about -json of the staged binary, refusing one that is not the official build of release.
func checkBuild(ctx context.Context, binary, release string) error {
	out, err := exec.CommandContext(ctx, binary, "about", "-json").Output()
	if err != nil {
		return fmt.Errorf("updater: refused the build of Gophenberg %s, its about -json failed: %w", release, err)
	}
	var about struct {
		Version string `json:"version"`
		Stamp   string `json:"stamp"`
	}
	_ = json.Unmarshal(out, &about)
	if about.Version != release || about.Stamp != officialStamp {
		return fmt.Errorf("updater: refused the build of Gophenberg %s, its about -json names version %q and stamp %q",
			release, about.Version, about.Stamp)
	}
	return nil
}

// backUp takes the backup of the database the switch needs, naming it in state.
func (r *run) backUp(ctx context.Context, state *State) error {
	name := backupName(r.running(), r.job.Check.Now())
	state.Backup = filepath.Join(backupsFolder, name)
	return Backup{
		PgDump: r.job.Settings.PgDump, DatabaseURL: r.job.DatabaseURL, Folder: r.layout.Backups(), Name: name,
		Keep: r.job.Settings.BackupsKept,
	}.Take(ctx)
}

// switchBuilds points previous at the running build and current at the new one, the state already saying switched.
func (r *run) switchBuilds(_ context.Context, state *State) error {
	if err := r.layout.Point(Previous, state.From); err != nil {
		return err
	}
	return r.layout.Point(Current, state.To)
}

// putBack deletes what the job wrote, points previous back and writes back the state it found, unless current moved.
func (r *run) putBack(err error) error {
	if r.switched() {
		return fmt.Errorf("updater: current names Gophenberg %s, so restart Gophenberg to start it: %w", r.to(), err)
	}
	r.removeFiles(r.to())
	return errors.Join(err, r.putPreviousBack(), r.layout.WriteState(r.held))
}

// switched reports whether the current link already names the build the job switches to, so the switch stands.
func (r *run) switched() bool {
	name, err := r.layout.Named(Current)
	return err == nil && name == r.to()
}

// putPreviousBack points previous back at the build it named before the job, or deletes the link the job made.
func (r *run) putPreviousBack() error {
	named, _ := r.layout.Named(Previous)
	switch {
	case named != r.running():
		return nil
	case r.previous == "":
		return os.Remove(filepath.Join(r.layout.Root, Previous))
	}
	return r.layout.Point(Previous, r.previous)
}

// clearInterrupted deletes the files of an earlier job that stopped before its switch ended, keeping what it refused.
func (r *run) clearInterrupted() {
	if !r.interrupted() {
		return
	}
	r.removeFiles(r.held.To)
	r.held = State{Refused: r.held.Refused}
}

// interrupted reports whether the state names an earlier job of this build that stopped before its switch ended.
func (r *run) interrupted() bool {
	switch {
	case !version.Valid(r.held.To) || r.held.To == r.running():
		return false
	case r.held.Step == StepSwitched:
		return r.held.From == r.running()
	}
	return slices.Contains(stoppedSteps, r.held.Step)
}

// removeFiles deletes the download and the build of release and any partial backup, what a job that fails leaves.
func (r *run) removeFiles(release string) {
	_ = os.Remove(r.layout.Download(release))
	_ = os.RemoveAll(r.layout.Build(release))
	partials, _ := filepath.Glob(filepath.Join(r.layout.Backups(), "*"+partialSuffix))
	for _, partial := range partials {
		_ = os.Remove(partial)
	}
}
