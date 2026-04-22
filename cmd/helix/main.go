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
	if len(args) == 0 || args[0] != "generate" {
		return fmt.Errorf("helix: expected subcommand generate")
	}

	restArgs := args[1:]
	if len(restArgs) > 0 && restArgs[0] == "wire" {
		return runGenerateWire(restArgs[1:])
	}
	return runGenerate(restArgs)
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
