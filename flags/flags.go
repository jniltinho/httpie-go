package flags

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/nojima/httpie-go/exchange"
	"github.com/nojima/httpie-go/input"
	"github.com/nojima/httpie-go/output"
	"github.com/nojima/httpie-go/version"
	"github.com/pborman/getopt"
	"github.com/pkg/errors"
)

var reNumber = regexp.MustCompile(`^[0-9.]+$`)

type Usage interface {
	PrintUsage(w io.Writer)
}

type OptionSet struct {
	InputOptions    input.Options
	ExchangeOptions exchange.Options
	OutputOptions   output.Options
}

type terminalInfo struct {
	stdinIsTerminal  bool
	stdoutIsTerminal bool
}

func Parse(args []string) ([]string, Usage, *OptionSet, error) {
	return parse(args, terminalInfo{
		stdinIsTerminal:  isatty.IsTerminal(os.Stdin.Fd()),
		stdoutIsTerminal: isatty.IsTerminal(os.Stdout.Fd()),
	})
}

func parse(args []string, terminalInfo terminalInfo) ([]string, Usage, *OptionSet, error) {
	inputOptions := input.Options{}
	outputOptions := output.Options{}
	exchangeOptions := exchange.Options{}
	var ignoreStdin bool
	var verboseFlag bool
	var headersFlag bool
	var bodyFlag bool
	printFlag := "\000" // "\000" is a special value that indicates user did not specified --print
	timeout := "30s"
	var prettyFlag string
	var versionFlag bool

	// Default value 20 is a bit too small for options of httpie-go.
	getopt.HelpColumn = 22

	flagSet := getopt.New()
	flagSet.SetParameters("[METHOD] URL [ITEM [ITEM ...]]")
	flagSet.BoolVarLong(&inputOptions.JSON, "json", 'j', "data items are serialized as JSON (default)")
	flagSet.BoolVarLong(&inputOptions.Form, "form", 'f', "data items are serialized as form fields")
	flagSet.StringVarLong(&printFlag, "print", 'p', "specifies what the output should contain (HBhb)")
	flagSet.BoolVarLong(&verboseFlag, "verbose", 'v', "print the request as well as the response. shortcut for --print=HBhb")
	flagSet.BoolVarLong(&headersFlag, "headers", 'h', "print only the request headers. shortcut for --print=h")
	flagSet.BoolVarLong(&bodyFlag, "body", 'b', "print only response body. shourtcut for --print=b")
	flagSet.BoolVarLong(&ignoreStdin, "ignore-stdin", 0, "do not attempt to read stdin")
	flagSet.StringVarLong(&timeout, "timeout", 0, "timeout seconds that you allow the whole operation to take")
	flagSet.StringVarLong(&prettyFlag, "pretty", 0, "controls output formatting (all, format, none)")
	flagSet.BoolVarLong(&exchangeOptions.FollowRedirects, "follow", 'F', "follow 30x Location redirects")
	flagSet.BoolVarLong(&versionFlag, "version", 0, "print version and exit")
	flagSet.Parse(args)

	// Check --version
	if versionFlag {
		fmt.Fprintf(os.Stderr, "httpie-go %s\n", version.Current())
		os.Exit(0)
	}

	// Check stdin
	if !ignoreStdin && !terminalInfo.stdinIsTerminal {
		inputOptions.ReadStdin = true
	}

	// Parse --print
	if err := parsePrintFlag(
		printFlag,
		verboseFlag,
		headersFlag,
		bodyFlag,
		terminalInfo.stdoutIsTerminal,
		&outputOptions,
	); err != nil {
		return nil, nil, nil, err
	}

	// Parse --timeout
	d, err := parseDurationOrSeconds(timeout)
	if err != nil {
		return nil, nil, nil, err
	}
	exchangeOptions.Timeout = d

	// Parse --pretty
	if err := parsePretty(prettyFlag, terminalInfo.stdoutIsTerminal, &outputOptions); err != nil {
		return nil, nil, nil, err
	}

	optionSet := &OptionSet{
		InputOptions:    inputOptions,
		ExchangeOptions: exchangeOptions,
		OutputOptions:   outputOptions,
	}
	return flagSet.Args(), flagSet, optionSet, nil
}

func parsePrintFlag(
	printFlag string,
	verboseFlag bool,
	headersFlag bool,
	bodyFlag bool,
	stdoutIsTerminal bool,
	outputOptions *output.Options,
) error {
	if printFlag == "\000" { // --print is not specified
		if headersFlag {
			outputOptions.PrintResponseHeader = true
		} else if bodyFlag {
			outputOptions.PrintResponseBody = true
		} else if verboseFlag {
			outputOptions.PrintRequestBody = true
			outputOptions.PrintRequestHeader = true
			outputOptions.PrintResponseHeader = true
			outputOptions.PrintResponseBody = true
		} else if stdoutIsTerminal {
			outputOptions.PrintResponseHeader = true
			outputOptions.PrintResponseBody = true
		} else {
			outputOptions.PrintResponseBody = true
		}
	} else { // --print is specified
		for _, c := range printFlag {
			switch c {
			case 'H':
				outputOptions.PrintRequestHeader = true
			case 'B':
				outputOptions.PrintRequestBody = true
			case 'h':
				outputOptions.PrintResponseHeader = true
			case 'b':
				outputOptions.PrintResponseBody = true
			default:
				return errors.Errorf("invalid char in --print value (must be consist of HBhb): %c", c)
			}
		}
	}
	return nil
}

func parsePretty(prettyFlag string, stdoutIsTerminal bool, outputOptions *output.Options) error {
	switch prettyFlag {
	case "":
		outputOptions.EnableFormat = stdoutIsTerminal
		outputOptions.EnableColor = stdoutIsTerminal
	case "all":
		outputOptions.EnableFormat = true
		outputOptions.EnableColor = true
	case "none":
		outputOptions.EnableFormat = false
		outputOptions.EnableColor = false
	case "format":
		outputOptions.EnableFormat = true
		outputOptions.EnableColor = false
	case "colors":
		return errors.New("--pretty=colors is not implemented")
	default:
		return errors.Errorf("unknown value of --pretty: %s", prettyFlag)
	}
	return nil
}

func parseDurationOrSeconds(timeout string) (time.Duration, error) {
	if reNumber.MatchString(timeout) {
		timeout += "s"
	}
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return time.Duration(0), errors.Errorf("Value of --timeout must be a number or duration string: %v", timeout)
	}
	return d, nil
}
