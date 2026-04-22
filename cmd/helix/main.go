package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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
		return fmt.Errorf("helix: expected subcommand new or generate")
	}

	switch args[0] {
	case "new":
		return runNew(args[1:])
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
		return fmt.Errorf("helix: expected subcommand new or generate")
	}
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
