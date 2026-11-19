// Command demo runs Prusa Connect with a fake fleet, in one process, so
// somebody can see this server working on a machine with no printer attached.
//
// It parses flags, installs signal handling and calls app.Demo. The printers
// are internal/printer/simulator.
//
// The server reads config/config.yaml when the file exists, with the demo's
// settings on top. It prints the account to sign in with.
//
// It is a separate command so that the simulator is not linked into the
// shipping binary.
//
//	go run ./cmd/demo
//	go run ./cmd/demo -printers 8 -flap 0
//	go run ./cmd/demo -config printers.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prusa3d/prusa-connect/internal/app"
	"github.com/prusa3d/prusa-connect/internal/kernel/procgroup"
)

// version is stamped at build time with -ldflags "-X main.version=...".
//
//nolint:gochecknoglobals // a build-stamped value has nowhere else to live.
var version string

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// flapDefault is the default of -flap. Zero means the fleet stays connected.
const flapDefault = time.Minute

// configDefault is the default of -config. When the file is missing the demo
// runs on its defaults.
const configDefault = "config/config.yaml"

func main() {
	// run returns the exit code, so its deferred calls run before os.Exit.
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("demo", flag.ContinueOnError)
	options := app.DemoOptions{Version: version}

	flags.StringVar(&options.APIAddr, "addr", "",
		"the address the core binds and the printers dial, defaulting to 127.0.0.1:8080")
	flags.StringVar(&options.Dir, "dir", "",
		"where the configuration and the database go, defaulting to a temporary directory that is removed")
	flags.StringVar(&options.ConfigPath, "config", configDefault,
		"the configuration file the demo lays its own settings over, run on the defaults when absent")
	flags.StringVar(&options.Examples, "examples", "",
		"where the packed example module releases are, defaulting to bin/demo-modules, which `make demo` writes")
	flags.IntVar(&options.Printers, "printers", 0, "how many printers dial in, defaulting to 3")
	flags.Func("models",
		"a comma-separated list of the printer models the fleet is drawn from, such as CoreOne_INDX, "+
			"defaulting to CoreOne,MK4S,XL",
		func(value string) error {
			for model := range strings.SplitSeq(value, ",") {
				if model = strings.TrimSpace(model); model != "" {
					options.Models = append(options.Models, model)
				}
			}

			return nil
		})
	flags.DurationVar(&options.Flap, "flap", flapDefault,
		"how often one printer goes away and comes back, zero to keep the whole fleet connected")
	flags.DurationVar(&options.PrintGap, "print-gap", 0,
		"the wait between a printer's prints, which also makes every print take seconds, zero for the demo's own pace")
	flags.Int64Var(&options.TransferRate, "transfer-rate", 0,
		"how many bytes a second each printer fetches a file at, zero for as fast as the server serves it")
	flags.DurationVar(&options.ListingDelay, "listing-delay", 0,
		"how long each printer takes to list a folder, holding its one command slot, zero to list at once")
	flags.BoolVar(&options.Idle, "idle", false,
		"start no print and raise no dialog, leaving the printers to a module that plans the prints")
	flags.BoolVar(&options.Cameras, "cameras", true,
		"start a simulated camera for each printer and declare it against that printer")
	flags.StringVar(&options.LogLevel, "log-level", "", "debug, info, warn or error, defaulting to error")
	flags.StringVar(&options.Marketplace, "marketplace", "",
		"the marketplace whose Connect modules the extensions page lists and installs, such as http://localhost:4455")
	flags.StringVar(&options.Password, "password", "",
		"the demo account's password, at least eight bytes, defaulting to a new one printed on every run")

	passwordFile := flags.String("password-file", "",
		"a file holding the demo account's password, which then stays out of the process list and the banner")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if *passwordFile != "" {
		if options.Password != "" {
			fmt.Fprintln(os.Stderr, "demo: -password and -password-file cannot both be set")

			return exitUsage
		}

		password, err := readPasswordFile(*passwordFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "demo: "+err.Error())

			return exitUsage
		}

		options.Password = password
		options.HidePassword = true
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// One reaper for the whole process, before any module starts.
	options.Reaper = procgroup.NewReaper()
	procgroup.KeepPortRightFromChildren()

	go options.Reaper.Run(ctx)

	if err := app.Demo(ctx, options); err != nil {
		fmt.Fprintln(os.Stderr, "demo: "+err.Error())

		return exitFailure
	}

	return exitOK
}

// readPasswordFile reads the password a file holds. The line ending an editor
// leaves after it is not part of the password.
func readPasswordFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read the password file: %w", err)
	}

	password := strings.TrimRight(string(raw), "\r\n")
	if password == "" {
		return "", fmt.Errorf("the password file %s is empty", path)
	}

	return password, nil
}
