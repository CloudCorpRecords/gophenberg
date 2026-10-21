// SPDX-License-Identifier: Apache-2.0

package updater

import "path/filepath"

// Status is what update:status shows of an install root: the builds its links name, the last update and the backups.
type Status struct {
	Root     string   `json:"root"`
	Current  string   `json:"current"`
	Previous string   `json:"previous"`
	Update   State    `json:"update"`
	Restore  string   `json:"restore"`
	Backups  []string `json:"backups"`
}

// ReadStatus returns the status of the install root the settings name, an empty one for an install with no root.
func ReadStatus(settings Settings) (Status, error) {
	if settings.Root == "" {
		return Status{Update: State{}.WithEmptyLists(), Backups: []string{}}, nil
	}
	layout := Layout{Root: settings.Root}
	state, err := layout.ReadState()
	if err != nil {
		return Status{}, err
	}
	status := Status{Root: layout.Root, Update: state.WithEmptyLists(), Backups: layout.backups()}
	status.Current, _ = layout.Named(Current)
	status.Previous, _ = layout.Named(Previous)
	if state.Step == StepFailed {
		status.Restore = layout.restoreText(state)
	}
	return status, nil
}

// backups returns each backup the backups folder keeps, relative to the install root, the newest first.
func (l Layout) backups() []string {
	dated := datedBackups(l.Backups())
	names := make([]string, 0, len(dated))
	for _, backup := range dated {
		names = append(names, filepath.Join(backupsFolder, backup.name))
	}
	return names
}
