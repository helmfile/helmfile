package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/kylelemons/godebug/pretty"
	"github.com/logrusorgru/aurora"
	"github.com/mattn/go-isatty"
	"gopkg.in/yaml.v3"
)

var version = "latest"

func main() {
	var opts struct {
		NoColor bool   `long:"no-color" description:"disable colored output" required:"false"`
		Version func() `long:"version" description:"print version and exit"`
		// Ignore  []string `long:"ignore" description:"ignore vaules by yaml path" required:"false"`
	}

	opts.Version = func() {
		fmt.Fprintf(os.Stderr, "%v\n", version)
		os.Exit(0)
	}

	args, err := flags.Parse(&opts)
	if err != nil {
		if err.(*flags.Error).Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if len(args) != 2 {
		fmt.Println("requires exactly two arguments to diff, use `yamldiff <file1> <file2>`")
		os.Exit(1)
	}

	formatter := newFormatter(opts.NoColor)

	errors := stat(args[0], args[1])
	failOnErr(formatter, errors...)

	yaml1, err := unmarshal(args[0])
	if err != nil {
		failOnErr(formatter, err)
	}
	yaml2, err := unmarshal(args[1])
	if err != nil {
		failOnErr(formatter, err)
	}

	diff, ndiff := computeDiff(formatter, yaml1, yaml2)
	if ndiff > 0 {
		fmt.Println(diff)
		os.Exit(2)
	}
}

func stat(filenames ...string) []error {
	var errs []error
	for _, filename := range filenames {
		if filename == "-" {
			continue
		}
		_, err := os.Stat(filename)
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot find file: %v. Does it exist?", filename))
		}
	}
	return errs
}

func unmarshal(filename string) (interface{}, error) {
	var contents []byte
	var err error
	if filename == "-" {
		contents, err = io.ReadAll(os.Stdin)
	} else {
		contents, err = os.ReadFile(filename)
	}
	if err != nil {
		return nil, err
	}
	var ret interface{}
	err = yaml.Unmarshal(contents, &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func failOnErr(formatter aurora.Aurora, errs ...error) {
	if len(errs) == 0 {
		return
	}
	var errMessages []string
	for _, err := range errs {
		errMessages = append(errMessages, err.Error())
	}
	fmt.Fprintf(os.Stderr, "%v\n\n", formatter.Red(strings.Join(errMessages, "\n")))
	os.Exit(1)
}

func computeDiff(formatter aurora.Aurora, a interface{}, b interface{}) (string, int) {
	diffs := make([]string, 0)
	ndiffs := 0
	for _, s := range strings.Split(pretty.Compare(a, b), "\n") {
		switch {
		case strings.HasPrefix(s, "+"):
			diffs = append(diffs, formatter.Bold(formatter.Green(s)).String())
			ndiffs++
		case strings.HasPrefix(s, "-"):
			diffs = append(diffs, formatter.Bold(formatter.Red(s)).String())
			ndiffs++
		default:
			diffs = append(diffs, s)
		}
	}
	return strings.Join(diffs, "\n"), ndiffs
}

func newFormatter(noColor bool) aurora.Aurora {
	var formatter aurora.Aurora
	if noColor || !isTerminal() {
		formatter = aurora.NewAurora(false)
	} else {
		formatter = aurora.NewAurora(true)
	}
	return formatter
}

func isTerminal() bool {
	fd := os.Stdout.Fd()
	switch {
	case isatty.IsTerminal(fd):
		return true
	case isatty.IsCygwinTerminal(fd):
		return true
	default:
		return false
	}
}
