package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/airencracken/folkweave/internal/folkweave"
	"golang.org/x/crypto/bcrypt"
)

func TestInitBackupRestoreLifecycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	var output bytes.Buffer
	password := "a-private-test-password"
	err := run(ctx, []string{"init", "--data", data, "--username", "owner", "--name", "Owner", "--title", "Our people", "--password-stdin"}, strings.NewReader(password+"\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), password) {
		t.Fatal("password printed")
	}
	store, err := folkweave.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Ready(ctx) {
		t.Fatal("owner not provisioned")
	}
	store.Close()
	if err = run(ctx, []string{"init", "--data", data, "--username", "other", "--password-stdin"}, strings.NewReader(password+"\n"), &output); err == nil {
		t.Fatal("initialization overwrote gathering")
	}
	snapshot := filepath.Join(root, "snapshot")
	if err = run(ctx, []string{"backup", "--data", data, "--output", snapshot}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(root, "restored")
	args := []string{"restore", "--from", filepath.Join(snapshot, "folkweave.db"), "--data", restored}
	if err = run(ctx, args, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	store, err = folkweave.Open(restored)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Ready(ctx) {
		t.Fatal("restored owner missing")
	}
	if err = store.Check(ctx); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err = run(ctx, args, strings.NewReader(""), &output); err == nil {
		t.Fatal("restore replaced existing data")
	}
	info, _ := os.Stat(filepath.Join(restored, "folkweave.db"))
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("database not private")
	}
}
func TestInvalidRestoreDoesNotPublishTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "bad.db")
	os.WriteFile(source, []byte("not a database"), 0600)
	destination := filepath.Join(root, "new")
	err := run(context.Background(), []string{"restore", "--from", source, "--data", destination}, strings.NewReader(""), io.Discard)
	if err == nil {
		t.Fatal("corrupt database restored")
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatal("failed restore published data")
	}
}
func TestHelpVersionAndInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"help"}, {"version"}, {"--version"}, {"init", "--help"}, {"serve", "--help"}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader(""), &out); err != nil || out.Len() == 0 {
			t.Fatal(args, err)
		}
	}
	for _, args := range [][]string{{"bad-command"}, {"serve", "--bad-flag"}, {"demo", "--listen", "0.0.0.0:8084"}, {"init", "--username", "bad!", "--password-stdin"}} {
		if err := run(context.Background(), args, strings.NewReader("a-private-test-password\n"), io.Discard); err == nil {
			t.Fatal("invalid invocation accepted", args)
		}
	}
}
func TestDemoUsesDisposableDataAndShutsDown(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("FOLKWEAVE_BACKUP_INTERVAL", "0")
	production := filepath.Join(root, "production")
	os.Mkdir(production, 0700)
	marker := filepath.Join(production, "keep")
	os.WriteFile(marker, []byte("untouched"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	finished := make(chan error, 1)
	go func() {
		finished <- run(ctx, []string{"demo", "--listen", "127.0.0.1:0", "--data", production}, strings.NewReader(""), writer)
		writer.Close()
	}()
	ready := make(chan struct{})
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := reader.Read(buffer)
			if n > 0 && strings.Contains(string(buffer[:n]), "listening at") {
				close(ready)
				io.Copy(io.Discard, reader)
				return
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("demo did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("demo did not stop")
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "untouched" {
		t.Fatal("demo touched production data")
	}
	files, _ := filepath.Glob(filepath.Join(root, "folkweave-demo-*"))
	if len(files) != 0 {
		t.Fatal("demo data not removed")
	}
}

func TestCreateOwnerCommand(t *testing.T) {
	ctx := context.Background()
	data := filepath.Join(t.TempDir(), "data")
	t.Setenv("FOLKWEAVE_DATA_DIR", data)
	password := "an-owner-test-password"
	var output bytes.Buffer
	if !instanceCommands["create-owner"] {
		t.Fatal("create-owner bypasses service-account re-execution and root-write protection")
	}
	for _, args := range [][]string{{"create-owner", "--username", "airen", "--name", "Airen", "--title", "Our gathering", "--password-stdin"}, {"create-owner", "--username", "second", "--password-stdin"}} {
		if err := run(ctx, args, strings.NewReader(password+"\n"), &output); err != nil {
			t.Fatal(args, err)
		}
	}
	store, err := folkweave.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, username := range []string{"airen", "second"} {
		u, hash, err := store.Credentials(ctx, username)
		if err != nil || !u.Owner() || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			t.Fatal("owner credentials failed", u, err)
		}
	}
	settings, err := store.Settings(ctx)
	if err != nil || settings.Title != "Our gathering" {
		t.Fatal("additional owner changed title", settings, err)
	}
	if strings.Contains(output.String(), password) || !strings.Contains(output.String(), "Owner account created.") {
		t.Fatal("unsafe or missing success output")
	}
	if err = run(ctx, []string{"create-owner", "--username", "airen", "--password-stdin"}, strings.NewReader("a-different-password\n"), io.Discard); err == nil {
		t.Fatal("duplicate command overwrote owner")
	}
}
func TestCreateOwnerHelpAndAdversarialArguments(t *testing.T) {
	ctx := context.Background()
	data := filepath.Join(t.TempDir(), "absent")
	t.Setenv("FOLKWEAVE_DATA_DIR", data)
	for _, args := range [][]string{{"--help"}, {"create-owner", "--help"}} {
		var out bytes.Buffer
		if err := run(ctx, args, strings.NewReader(""), &out); err != nil || !strings.Contains(out.String(), "create-owner") {
			t.Fatal(args, err, out.String())
		}
	}
	for _, args := range [][]string{{"create-owner", "--username", "bad!", "--password-stdin"}, {"create-owner", "--username", "valid", "--password-stdin"}, {"create-owner", "--username", "valid", "unexpected", "--password-stdin"}, {"create-owner", "--unknown-flag"}, {"create-owner", "--username", "valid"}} {
		if err := run(ctx, args, strings.NewReader("short\n"), io.Discard); err == nil {
			t.Fatal("invalid invocation accepted", args)
		}
	}
	if _, err := os.Stat(data); !os.IsNotExist(err) {
		t.Fatal("help or invalid input created data", err)
	}
}
