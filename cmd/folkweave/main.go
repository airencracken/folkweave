package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/airencracken/comfylib/autobackup"
	"github.com/airencracken/comfylib/password"
	"github.com/airencracken/comfylib/privdrop"
	"github.com/airencracken/comfylib/svcconfig"
	"github.com/airencracken/folkweave/internal/folkweave"
	"github.com/gofrs/flock"
)

var version = "0.7.2"

var instanceCommands = map[string]bool{"setup": true, "init": true, "create-owner": true, "set-password": true, "backup": true, "restore": true}
var serviceSettings = []string{"FOLKWEAVE_LISTEN", "FOLKWEAVE_BASE_URL", "FOLKWEAVE_TRUSTED_PROXIES", "FOLKWEAVE_BACKUP_INTERVAL", "FOLKWEAVE_BACKUP_KEEP", "FOLKWEAVE_BACKUP_MAX_BYTES", "FOLKWEAVE_BACKUP_COPY_MAX_BYTES"}

func main() {
	paths := svcconfig.Detect("folkweave", "/var/lib/folkweave")
	handled, status, err := privdrop.Reexec(privdrop.Request{Args: os.Args[1:], Commands: instanceCommands, Paths: paths, DefaultUser: "folkweave", Settings: func(command, dir string) (map[string]string, error) { return paths.Settings(serviceSettings...) }})
	if handled {
		if err != nil {
			slog.Error("administrative command", "error", err)
		}
		os.Exit(status)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		slog.Error("folkweave", "error", err)
		os.Exit(1)
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	if len(args) > 0 && args[0] == "help" && len(args) == 2 {
		return run(ctx, []string{args[1], "--help"}, in, out)
	}
	if len(args) > 2 && args[0] == "help" {
		return errors.New("usage: folkweave help [COMMAND]")
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, "Folkweave — a private timeline for your people.\n\nCommands: setup, create-owner, serve, demo, backup, restore, set-password, version\nUse COMMAND --help for options.")
		return nil
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(out, "folkweave", version)
		return nil
	}
	command := args[0]
	if instanceCommands[command] && !privdrop.HasHelpFlag(args[1:]) {
		if err := privdrop.RefuseRoot(os.Geteuid(), command, "sudo -u folkweave folkweave "+command); err != nil {
			return err
		}
	}
	paths := svcconfig.Detect("folkweave", "/var/lib/folkweave")
	dataDefault := "./data"
	settings := map[string]string{}
	if command != "demo" {
		var err error
		dataDefault, err = paths.DataDir("FOLKWEAVE_DATA_DIR")
		if err != nil {
			return err
		}
		settings, err = processSettings(paths)
		if err != nil {
			return err
		}
	}
	setting := func(key, fallback string) string {
		if v := settings[key]; v != "" {
			return v
		}
		return env(key, fallback)
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(out)
	data := f.String("data-dir", dataDefault, "private data directory")
	// --data is an undocumented alias kept for existing scripts.
	f.StringVar(data, "data", dataDefault, "")
	f.Usage = func() { usage(f, command) }
	switch command {
	case "setup":
		if err := f.Parse(args[1:]); err != nil {
			return helpError(err)
		}
		if f.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		store, err := folkweave.Open(*data)
		if err != nil {
			return err
		}
		defer store.Close()
		wizard, err := firstSetup(store, *data, setting("FOLKWEAVE_BASE_URL", ""), setting("FOLKWEAVE_LISTEN", "127.0.0.1:8084"))
		if err != nil {
			return err
		}
		return wizard.Enable(ctx, out)
	case "init", "create-owner", "set-password":
		username := f.String("username", "", "account username")
		name := f.String("name", "", "owner's display name")
		title := f.String("title", "Our people", "group name, used only when creating the first owner")
		source := password.Flags(f)
		if err := f.Parse(args[1:]); err != nil {
			return helpError(err)
		}
		if f.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		secret, err := source.Read(in, out)
		if err != nil {
			return err
		}
		*username = strings.ToLower(strings.TrimSpace(*username))
		if err = folkweave.ValidateCredentials(*username, secret); err != nil {
			return err
		}
		if *name == "" {
			*name = *username
		}
		hash, err := folkweave.HashPassword(secret)
		if err != nil {
			return err
		}
		store, err := folkweave.Open(*data)
		if err != nil {
			return err
		}
		defer store.Close()
		if command == "set-password" {
			if err = store.SetPassword(ctx, *username, hash); err != nil {
				return err
			}
			fmt.Fprintln(out, "Password changed; earlier sessions have been revoked.")
			return nil
		}
		provision := store.Bootstrap
		if command == "create-owner" {
			provision = store.CreateOwner
		}
		if err = provision(ctx, *username, hash, *name, *title); err != nil {
			return err
		}
		if command == "create-owner" {
			fmt.Fprintln(out, "Owner account created.")
		} else {
			fmt.Fprintln(out, "Your group is ready. Start it with: folkweave serve --data-dir", *data)
		}
		return nil
	case "serve", "demo":
		listen := f.String("listen", setting("FOLKWEAVE_LISTEN", "127.0.0.1:8084"), "HTTP listen address")
		base := f.String("base-url", setting("FOLKWEAVE_BASE_URL", ""), "public HTTP or HTTPS origin")
		trusted := f.String("trusted-proxies", setting("FOLKWEAVE_TRUSTED_PROXIES", ""), "comma-separated trusted proxy CIDRs")
		if err := f.Parse(args[1:]); err != nil {
			return helpError(err)
		}
		if f.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		if command == "demo" {
			host, _, err := net.SplitHostPort(*listen)
			if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
				return errors.New("demo must listen on a loopback IP address")
			}
			temporary, err := os.MkdirTemp("", "folkweave-demo-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(temporary)
			*data = temporary
		}
		store, err := folkweave.Open(*data)
		if err != nil {
			return err
		}
		defer store.Close()
		lock := flock.New(filepath.Join(*data, "server.lock"))
		locked, err := lock.TryLock()
		if err != nil {
			return err
		}
		if !locked {
			return errors.New("another Folkweave server is using this data directory")
		}
		defer lock.Unlock()
		if command == "demo" {
			if err = folkweave.SeedDemo(ctx, store); err != nil {
				return err
			}
		}
		listener, err := net.Listen("tcp", *listen)
		if err != nil {
			return err
		}
		defer listener.Close()
		if *base == "" {
			*base = "http://" + listener.Addr().String()
		}
		app, err := folkweave.NewApp(store, folkweave.Config{Version: version, BaseURL: *base, TrustedProxies: *trusted})
		if err != nil {
			return err
		}
		backupConfig, err := autobackup.Load("folkweave", *data, func(key string) string { return setting(key, "") })
		if err != nil {
			return err
		}
		app.Backups, err = autobackup.New(backupConfig, store.Snapshot, slog.Default())
		if err != nil {
			return err
		}
		wizard, err := firstSetup(store, *data, *base, *listen)
		if err != nil {
			return err
		}
		workerCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			if wizard.Wait(workerCtx) == nil {
				app.Backups.Run(workerCtx)
			}
		}()
		defer func() { cancel(); <-done }()
		server := &http.Server{Handler: wizard.Wrap(app), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32 << 10}
		finished := make(chan error, 1)
		go func() { finished <- server.Serve(listener) }()
		fmt.Fprintln(out, "Folkweave is listening at", *base)
		if command == "demo" {
			fmt.Fprintln(out, "Disposable demo with fictional people and photographs.\nUsername: demo\nPassword: folkweave-demo-only")
		}
		select {
		case err = <-finished:
			if flushErr := app.Flush(context.Background()); flushErr != nil {
				slog.Warn("save request counts", "error", flushErr)
			}
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-ctx.Done():
			shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			err = server.Shutdown(shutdown)
			if err != nil {
				server.Close()
			}
			if flushErr := app.Flush(shutdown); flushErr != nil {
				slog.Warn("save request counts", "error", flushErr)
			}
			return err
		}
	case "backup":
		destination := f.String("output", "", "new snapshot directory")
		limit := f.Int64("max-bytes", 5<<30, "snapshot byte limit")
		if err := f.Parse(args[1:]); err != nil {
			return helpError(err)
		}
		if *destination == "" || *limit < 1 || f.NArg() != 0 {
			return errors.New("choose --output and a positive --max-bytes")
		}
		if _, err := os.Lstat(*destination); !errors.Is(err, os.ErrNotExist) {
			return errors.New("snapshot destination must not already exist")
		}
		store, err := folkweave.Open(*data)
		if err != nil {
			return err
		}
		defer store.Close()
		if !store.Ready(ctx) {
			return errors.New("the source is not an initialized Folkweave group")
		}
		if err = store.Snapshot(ctx, *destination, *limit); err != nil {
			return err
		}
		fmt.Fprintln(out, "Verified snapshot:", filepath.Join(*destination, "folkweave.db"))
		return nil
	case "restore":
		source := f.String("from", "", "verified snapshot database")
		if err := f.Parse(args[1:]); err != nil {
			return helpError(err)
		}
		if *source == "" || f.NArg() != 0 {
			return errors.New("choose --from snapshot/folkweave.db and a fresh --data-dir directory")
		}
		return restore(ctx, *source, *data, out)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

// hiddenFlags stay accepted but are left out of help output.
var hiddenFlags = map[string]bool{"data": true}

func usage(f *flag.FlagSet, command string) {
	out := f.Output()
	fmt.Fprintf(out, "Usage: folkweave %s [options]\n\nOptions:\n", command)
	f.VisitAll(func(option *flag.Flag) {
		if hiddenFlags[option.Name] {
			return
		}
		name, help := flag.UnquoteUsage(option)
		if name != "" {
			name = " " + name
		}
		fmt.Fprintf(out, "  --%s%s\n    \t%s", option.Name, name, help)
		if option.DefValue != "" && option.DefValue != "false" {
			fmt.Fprintf(out, " (default %q)", option.DefValue)
		}
		fmt.Fprintln(out)
	})
}

func helpError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}
func restore(ctx context.Context, source, destination string, out io.Writer) error {
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("restore destination must not exist")
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 5<<30 {
		return errors.New("choose a regular snapshot database up to 5 GiB")
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(abs), ".folkweave-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(filepath.Join(stage, "folkweave.db"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(output, io.LimitReader(input, (5<<30)+1))
	syncErr := output.Sync()
	closeErr := output.Close()
	if err = errors.Join(copyErr, syncErr, closeErr); err != nil {
		return err
	}
	if n > 5<<30 {
		return errors.New("snapshot exceeds 5 GiB")
	}
	store, err := folkweave.Open(stage)
	if err != nil {
		return err
	}
	if !store.Ready(ctx) {
		store.Close()
		return errors.New("snapshot is not an initialized Folkweave database")
	}
	if err = store.Check(ctx); err != nil {
		store.Close()
		return err
	}
	if err = store.Close(); err != nil {
		return err
	}
	// The freshly created target prevents rename from replacing an existing directory.
	if err = os.Mkdir(abs, 0700); err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(stage, "folkweave.db"), filepath.Join(abs, "folkweave.db")); err != nil {
		os.Remove(abs)
		return err
	}
	fmt.Fprintln(out, "Restored group:", abs)
	return nil
}

// Service managers and root administrative re-execution already supply the
// resolved environment. A service account must not reopen root-only config.
func processSettings(paths svcconfig.Paths) (map[string]string, error) {
	if os.Geteuid() == 0 {
		return paths.Settings(serviceSettings...)
	}
	settings := map[string]string{}
	for _, key := range serviceSettings {
		if value := os.Getenv(key); value != "" {
			settings[key] = value
		}
	}
	return settings, nil
}
