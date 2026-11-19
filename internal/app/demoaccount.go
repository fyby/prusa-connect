package app

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/prusa3d/prusa-connect/internal/platform/auth/local"
)

// The demo account. The username is fixed so the lines the demo prints can be
// copied from one run to the next; the password is new on every run unless
// the caller set one, and it is printed once, on the terminal the demo owns.
const (
	demoUsername    = "demo"
	demoDisplayName = "Demo"
	// demoPasswordBytes is how much randomness a generated password carries.
	// Twelve bytes is 96 bits, and twenty characters once encoded.
	demoPasswordBytes = 12
	// demoCredentialName is the label the demo mints its own token under. It
	// is in the listing at GET /api/v1/auth/credentials beside whatever the
	// person mints, which is how they can tell the two apart.
	demoCredentialName = "demo"
)

// errDemoAccountMissing is a kept directory whose store holds accounts and no
// demo account among them. The demo cannot make one that holds a grant, so it
// says what it found rather than serving a login that can do nothing.
var errDemoAccountMissing = errors.New("the directory holds accounts and none of them is the demo account")

// demoAccount writes the demo account into the store the core is about to
// open, before the core opens it, and returns its password.
//
// It runs before the core rather than through POST /api/v1/auth/setup, for
// the reason `admin create` exists: the setup path is open to whoever reaches
// it first, and a demo that started its listener and then set itself up would
// hold that window open on every run. A fresh directory gets the first
// administrator, which is the account the graph's first grant was written
// for. A kept directory already holds it, so the password is replaced, the
// sessions and tokens of the last run end, and the account keeps its grants.
func demoAccount(ctx context.Context, path string, opts DemoOptions) (string, error) {
	password := opts.Password

	if password == "" {
		generated, err := randomPassword()
		if err != nil {
			return "", err
		}

		password = generated
	}

	accounts, closeHost, err := openAccountsFor(ctx, path, false)
	if err != nil {
		return "", fmt.Errorf("open the demo account store: %w", err)
	}

	defer closeHost()

	first, err := accounts.store.NeedsSetup(ctx)
	if err != nil {
		return "", fmt.Errorf("read the demo accounts: %w", err)
	}

	spec := local.NewAccount{Username: demoUsername, DisplayName: demoDisplayName, Password: []byte(password)}

	if first {
		if _, err = accounts.store.CreateFirst(ctx, accounts.firstID, spec); err != nil {
			return "", fmt.Errorf("create the demo account: %w", err)
		}

		return password, nil
	}

	found, err := accountNamed(ctx, accounts, demoUsername)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errDemoAccountMissing, err)
	}

	if err = accounts.store.SetPassword(ctx, found.ID, []byte(password)); err != nil {
		return "", fmt.Errorf("set the demo account's password: %w", err)
	}

	// A new password ends what the old one handed out, as on every other path
	// that changes a password.
	if _, err = revokeAccountCredentials(ctx, accounts.credentials, accounts.pairings, found.ID); err != nil {
		return "", fmt.Errorf("end the demo account's old sessions and tokens: %w", err)
	}

	return password, nil
}

// randomPassword is a password nobody chose: random bytes, encoded so it can
// be typed and pasted without quoting.
func randomPassword() (string, error) {
	raw := make([]byte, demoPasswordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate the demo password: %w", err)
	}

	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)), nil
}

// demoLogin is what the demo signs in with, and what it prints.
type demoLogin struct {
	username string
	password string
	// hidden is a password the banner names by where it came from instead
	// of printing it.
	hidden bool
}

// hiddenPassword is what the banner prints in place of a hidden password.
const hiddenPassword = "<the password in the -password-file file>"

// shown is the login as the banner prints it.
func (l demoLogin) shown() demoLogin {
	if l.hidden {
		l.password = hiddenPassword
	}

	return l
}

// body is the JSON both login paths take.
func (l demoLogin) body() string {
	return fmt.Sprintf(`{"username":%q,"password":%q}`, l.username, l.password)
}

// demoToken is the part of a minted credential the demo reads back.
type demoToken struct {
	Token string `json:"token"`
}

// signIn logs the demo account in and mints a token for the demo's own reads,
// over the same two paths a person uses. The session lives in a cookie jar
// this call owns, so every request after it carries the token and nothing
// else, which is the shape a script or a page sends.
func (f *demoFleet) signIn(ctx context.Context, opts DemoOptions, login demoLogin) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("hold the demo session: %w", err)
	}

	session := &demoFleet{client: &http.Client{Timeout: demoRequest, Jar: jar}}

	status, err := session.call(ctx, http.MethodPost, opts.baseURL()+"/api/v1/auth/login", login.body(), nil)
	if err != nil {
		return "", fmt.Errorf("log the demo account in: %w", err)
	}

	if status != http.StatusOK {
		return "", fmt.Errorf("the demo account's login was refused with status %d", status)
	}

	var minted demoToken

	status, err = session.call(ctx, http.MethodPost, opts.baseURL()+"/api/v1/auth/credentials",
		fmt.Sprintf(`{"name":%q,"scopes":["administer"]}`, demoCredentialName), &minted)
	if err != nil {
		return "", fmt.Errorf("mint the demo token: %w", err)
	}

	if status != http.StatusCreated || minted.Token == "" {
		return "", fmt.Errorf("the demo token was refused with status %d", status)
	}

	return minted.Token, nil
}
