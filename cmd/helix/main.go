package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/enokdev/helix/cli"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix: expected subcommand new, db, or generate")
	}

	switch args[0] {
	case "new":
		return runNew(args[1:])
	case "db":
		return runDB(args[1:])
	case "generate":
		restArgs := args[1:]
		if len(restArgs) > 0 && restArgs[0] == "wire" {
			return runGenerateWire(restArgs[1:])
		}
		if len(restArgs) > 0 && restArgs[0] == "module" {
			return runGenerateModule(restArgs[1:])
		}
		if len(restArgs) > 0 && restArgs[0] == "context" {
			return runGenerateContext(restArgs[1:])
		}
		return runGenerate(restArgs)
	default:
		return fmt.Errorf("helix: expected subcommand new, db, or generate")
	}
}

func runDB(args []string) error {
	if len(args) == 0 || args[0] != "migrate" {
		return fmt.Errorf("helix db: expected subcommand migrate")
	}
	return runDBMigrate(args[1:])
}

func runDBMigrate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix db migrate: expected subcommand create, up, down, or status")
	}
	switch args[0] {
	case "create":
		return runDBMigrateCreate(args[1:])
	case "up":
		return runDBMigrateUp(args[1:])
	case "down":
		return runDBMigrateDown(args[1:])
	case "status":
		return runDBMigrateStatus(args[1:])
	default:
		return fmt.Errorf("helix db migrate: expected subcommand create, up, down, or status")
	}
}

func runDBMigrateCreate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix db migrate create: expected migration name")
	}
	if strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("helix db migrate create: migration name must come before flags (got %q)", args[0])
	}
	name := args[0]
	flags := flag.NewFlagSet("db migrate create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix db migrate create: unexpected argument %q", flags.Arg(0))
	}
	return cli.CreateMigration(context.Background(), cli.CreateMigrationOptions{Dir: *dir, Name: name})
}

func runDBMigrateUp(args []string) error {
	opts, err := parseDBMigrateOptions("db migrate up", args)
	if err != nil {
		return err
	}
	opts.Output = os.Stdout
	return cli.MigrateUp(context.Background(), opts)
}

func runDBMigrateDown(args []string) error {
	opts, err := parseDBMigrateOptions("db migrate down", args)
	if err != nil {
		return err
	}
	opts.Output = os.Stdout
	return cli.MigrateDown(context.Background(), opts)
}

func runDBMigrateStatus(args []string) error {
	opts, err := parseDBMigrateOptions("db migrate status", args)
	if err != nil {
		return err
	}
	opts.Output = os.Stdout
	return cli.MigrationStatus(context.Background(), opts)
}

func parseDBMigrateOptions(name string, args []string) (cli.MigrationOptions, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	databaseURL := flags.String("database-url", "", "database URL override")
	if err := flags.Parse(args); err != nil {
		return cli.MigrationOptions{}, err
	}
	if flags.NArg() != 0 {
		return cli.MigrationOptions{}, fmt.Errorf("helix %s: unexpected argument %q", name, flags.Arg(0))
	}
	return cli.MigrationOptions{Dir: *dir, DatabaseURL: *databaseURL}, nil
}

func runNew(args []string) error {
	if len(args) == 0 || args[0] != "app" {
		return fmt.Errorf("helix new: expected subcommand app")
	}
	return runNewApp(args[1:])
}

func runNewApp(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix new app: expected app name")
	}
	name := args[0]
	flags := flag.NewFlagSet("new app", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "directory where the app folder is created")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix new app: unexpected argument %q", flags.Arg(0))
	}
	return cli.NewApp(context.Background(), cli.NewAppOptions{Dir: *dir, Name: name})
}

func runGenerate(args []string) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "directory tree to scan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix generate: unexpected argument %q", flags.Arg(0))
	}
	return cli.Generate(context.Background(), cli.GenerateOptions{Dir: *dir})
}

func runGenerateWire(args []string) error {
	flags := flag.NewFlagSet("generate wire", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "directory tree to scan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix generate wire: unexpected argument %q", flags.Arg(0))
	}
	return cli.GenerateWire(context.Background(), cli.GenerateWireOptions{Dir: *dir})
}

func runGenerateModule(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix generate module: expected module name")
	}
	name := args[0]
	flags := flag.NewFlagSet("generate module", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix generate module: unexpected argument %q", flags.Arg(0))
	}
	return cli.GenerateModule(context.Background(), cli.GenerateModuleOptions{Dir: *dir, Name: name})
}

func runGenerateContext(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix generate context: expected context name")
	}
	name := args[0]
	flags := flag.NewFlagSet("generate context", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix generate context: unexpected argument %q", flags.Arg(0))
	}
	return cli.GenerateContext(context.Background(), cli.GenerateContextOptions{Dir: *dir, Name: name})
}
