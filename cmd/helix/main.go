package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/enokdev/helix/cli"
)

var cliStdout io.Writer = os.Stdout

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix: expected subcommand new, db, generate, run, or build")
	}

	switch args[0] {
	case "new":
		return runNew(ctx, args[1:])
	case "db":
		return runDB(ctx, args[1:])
	case "run":
		return runRun(ctx, args[1:])
	case "build":
		return runBuild(ctx, args[1:])
	case "generate":
		restArgs := args[1:]
		if len(restArgs) > 0 && restArgs[0] == "wire" {
			return runGenerateWire(ctx, restArgs[1:])
		}
		if len(restArgs) > 0 && restArgs[0] == "module" {
			return runGenerateModule(ctx, restArgs[1:])
		}
		if len(restArgs) > 0 && restArgs[0] == "context" {
			return runGenerateContext(ctx, restArgs[1:])
		}
		return runGenerate(ctx, restArgs)
	default:
		return fmt.Errorf("helix: expected subcommand new, db, generate, run, or build")
	}
}

func runDB(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "migrate" {
		return fmt.Errorf("helix db: expected subcommand migrate")
	}
	return runDBMigrate(ctx, args[1:])
}

func runDBMigrate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("helix db migrate: expected subcommand create, up, down, or status")
	}
	switch args[0] {
	case "create":
		return runDBMigrateCreate(ctx, args[1:])
	case "up":
		return runDBMigrateUp(ctx, args[1:])
	case "down":
		return runDBMigrateDown(ctx, args[1:])
	case "status":
		return runDBMigrateStatus(ctx, args[1:])
	default:
		return fmt.Errorf("helix db migrate: expected subcommand create, up, down, or status")
	}
}

func runDBMigrateCreate(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("db migrate create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	name, err := parseNamedCommand(flags, args, "migration name")
	if err != nil {
		return err
	}
	return cli.CreateMigration(ctx, cli.CreateMigrationOptions{Dir: *dir, Name: name})
}

func runDBMigrateUp(ctx context.Context, args []string) error {
	opts, err := parseDBMigrateOptions("db migrate up", args)
	if err != nil {
		return err
	}
	opts.Output = cliStdout
	return cli.MigrateUp(ctx, opts)
}

func runDBMigrateDown(ctx context.Context, args []string) error {
	opts, err := parseDBMigrateOptions("db migrate down", args)
	if err != nil {
		return err
	}
	opts.Output = cliStdout
	return cli.MigrateDown(ctx, opts)
}

func runDBMigrateStatus(ctx context.Context, args []string) error {
	opts, err := parseDBMigrateOptions("db migrate status", args)
	if err != nil {
		return err
	}
	opts.Output = cliStdout
	return cli.MigrationStatus(ctx, opts)
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

func runNew(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "app" {
		return fmt.Errorf("helix new: expected subcommand app")
	}
	return runNewApp(ctx, args[1:])
}

func runRun(ctx context.Context, args []string) error {
	opts, err := parseRunOptions(args)
	if err != nil {
		return err
	}
	return cli.Run(ctx, opts)
}

func parseRunOptions(args []string) (cli.RunOptions, error) {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	if err := flags.Parse(args); err != nil {
		return cli.RunOptions{}, err
	}
	return cli.RunOptions{Dir: *dir, Args: flags.Args()}, nil
}

func runBuild(ctx context.Context, args []string) error {
	opts, err := parseBuildOptions(args)
	if err != nil {
		return err
	}
	return cli.Build(ctx, opts)
}

func parseBuildOptions(args []string) (cli.BuildOptions, error) {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	docker := flags.Bool("docker", false, "generate a Dockerfile")
	if err := flags.Parse(args); err != nil {
		return cli.BuildOptions{}, err
	}
	if flags.NArg() != 0 {
		return cli.BuildOptions{}, fmt.Errorf("helix build: unexpected argument %q", flags.Arg(0))
	}
	return cli.BuildOptions{Dir: *dir, Docker: *docker}, nil
}

func runNewApp(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("new app", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "directory where the app folder is created")
	name, err := parseNamedCommand(flags, args, "app name")
	if err != nil {
		return err
	}
	return cli.NewApp(ctx, cli.NewAppOptions{Dir: *dir, Name: name})
}

func runGenerate(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "directory tree to scan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix generate: unexpected argument %q", flags.Arg(0))
	}
	return cli.Generate(ctx, cli.GenerateOptions{Dir: *dir, Output: cliStdout})
}

func runGenerateWire(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("generate wire", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "directory tree to scan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("helix generate wire: unexpected argument %q", flags.Arg(0))
	}
	return cli.GenerateWire(ctx, cli.GenerateWireOptions{Dir: *dir})
}

func runGenerateModule(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("generate module", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	name, err := parseNamedCommand(flags, args, "module name")
	if err != nil {
		return err
	}
	return cli.GenerateModule(ctx, cli.GenerateModuleOptions{Dir: *dir, Name: name})
}

func runGenerateContext(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("generate context", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", ".", "Go module root")
	name, err := parseNamedCommand(flags, args, "context name")
	if err != nil {
		return err
	}
	return cli.GenerateContext(ctx, cli.GenerateContextOptions{Dir: *dir, Name: name})
}

func parseNamedCommand(flags *flag.FlagSet, args []string, nameDescription string) (string, error) {
	var flagArgs []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	if err := flags.Parse(flagArgs); err != nil {
		return "", err
	}
	if len(positionals) == 0 {
		return "", fmt.Errorf("helix %s: expected %s", flags.Name(), nameDescription)
	}
	if len(positionals) > 1 {
		return "", fmt.Errorf("helix %s: unexpected argument %q", flags.Name(), positionals[1])
	}
	return positionals[0], nil
}
