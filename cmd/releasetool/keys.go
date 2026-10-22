// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"cmp"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
)

// keyListValidity is how long a key list stays valid when -valid names no other span.
const keyListValidity = 8760 * time.Hour

// listedRoles are the roles keys writes, in the order it lists them.
var listedRoles = []update.Role{update.RoleRelease, update.RoleStandby, update.RoleTimestamp}

// keys writes a key list naming the release key, the standby and the timestamp key, for a root to sign.
func keys(args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("keys", stderr)
	named := keyFlags(flags)
	serial := flags.Int64("serial", 0, "the serial, the Unix time of the run when left out")
	valid := flags.Duration("valid", keyListValidity, "how long the key list stays valid")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if missingKey(named) {
		return usageError{"keys needs -release, -standby and -timestamp, the Multikeys of the release key, the standby " +
			"and the timestamp key"}
	}
	if *valid <= 0 || flags.NArg() != 0 {
		return usageError{"keys needs a -valid above zero and reads no file"}
	}
	listed, err := listedKeys(named)
	if err != nil {
		return err
	}
	now := time.Now()
	return writeJSON(stdout, update.KeyList{
		Schema: 1, Serial: cmp.Or(*serial, now.Unix()), Expires: now.Add(*valid).UTC().Truncate(time.Second), Keys: listed,
	})
}

// listedKeys returns the keys of each listed role in order, refusing one that is not Ed25519 or holds two roles.
func listedKeys(named map[update.Role]*string) ([]update.ListedKey, error) {
	listed := make([]update.ListedKey, 0, len(listedRoles))
	held := map[string]update.Role{}
	for _, role := range listedRoles {
		key, err := signature.ParseMultikey(*named[role])
		public, isEd25519 := key.(ed25519.PublicKey)
		if err != nil || !isEd25519 {
			return nil, fmt.Errorf("refused -%s %q, which is not the Multikey of an Ed25519 key", role, *named[role])
		}
		if other, twice := held[string(public)]; twice {
			return nil, fmt.Errorf("refused -%s, the same key as -%s", role, other)
		}
		held[string(public)] = role
		listed = append(listed, update.ListedKey{Role: role, Multikey: *named[role]})
	}
	return listed, nil
}

// signKeys adds the signature line of a root key to the .sig beside a key list.
func signKeys(args []string, stdout, stderr io.Writer) error {
	var opts signOptions
	flags := newFlagSet("sign-keys", stderr)
	flags.StringVar(&opts.key, "key", "", "the root key file")
	flags.StringVar(&opts.passphraseFile, "passphrase-file", "", "a file whose first line opens a sealed root key")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if opts.key == "" || flags.NArg() != 1 {
		return usageError{"sign-keys needs -key, a root key file, and the one key list to sign"}
	}
	root, err := opts.rootKey()
	if err != nil {
		return err
	}
	list := releaseFile(flags.Arg(0))
	line, err := list.rootLine(root)
	if err != nil {
		return err
	}
	if err := list.addSignatureLine(line); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "signed %s as %s\n", list, strings.Fields(line)[0])
	return nil
}

// rootKey reads the root key file, refusing every key but P-256.
func (o signOptions) rootKey() (*ecdsa.PrivateKey, error) {
	key, err := o.signer("root key")
	if err != nil {
		return nil, err
	}
	root, isECDSA := key.(*ecdsa.PrivateKey)
	if !isECDSA || root.Curve != elliptic.P256() {
		return nil, fmt.Errorf("refused %s, which holds no P-256 key: a root key is P-256", o.key)
	}
	return root, nil
}

// rootLine returns the line the root signs the key list with, refusing a .sig the line does not belong beside.
func (f releaseFile) rootLine(root *ecdsa.PrivateKey) (string, error) {
	body, err := os.ReadFile(string(f))
	if err != nil {
		return "", fmt.Errorf("reading the key list: %w", err)
	}
	line, _ := update.SignKeyList(body, root)
	held, err := os.ReadFile(f.signaturePath())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("reading the signatures of the key list: %w", err)
	}
	ours := signatureLine(strings.Fields(line))
	for text := range strings.SplitSeq(strings.TrimSpace(string(held)), "\n") {
		if err := f.checkSignatureLine(body, signatureLine(strings.Fields(text)), ours); err != nil {
			return "", err
		}
	}
	return line, nil
}

// signatureLine holds the words of one line of a .sig: the Multikey of a root, then its signature.
type signatureLine []string

// checkSignatureLine refuses a line of the .sig that does not sign body, or that the root of ours already wrote.
func (f releaseFile) checkSignatureLine(body []byte, held, ours signatureLine) error {
	if len(held) == 0 {
		return nil
	}
	if !held.signs(body) {
		return fmt.Errorf("refused to sign %s: a line of its signatures does not sign it, so remove %s and sign "+
			"again with each root", f, f.signaturePath())
	}
	if held[0] == ours[0] {
		return fmt.Errorf("refused to sign %s again: the root %s already signed it", f, ours[0])
	}
	return nil
}

// signs reports whether the line is a P-256 key and its signature of body.
func (l signatureLine) signs(body []byte) bool {
	if len(l) != 2 {
		return false
	}
	key, _ := signature.ParseMultikey(l[0])
	public, isECDSA := key.(*ecdsa.PublicKey)
	return isECDSA && signature.VerifyCompact(public, body, l[1]) == nil
}

// addSignatureLine adds line to the end of the file's .sig, making the .sig when there is none.
func (f releaseFile) addSignatureLine(line string) error {
	file, err := os.OpenFile(f.signaturePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("writing %s: %w", f.signaturePath(), err)
	}
	_, err = file.WriteString(line + "\n")
	return errors.Join(err, file.Close())
}

// writeJSON writes v to w as indented JSON ending in a newline.
func writeJSON(w io.Writer, v any) error {
	body, _ := json.MarshalIndent(v, "", "  ")
	_, err := io.Copy(w, bytes.NewReader(append(body, '\n')))
	return err
}
