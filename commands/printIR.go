package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli"

	"github.com/StackExchange/dnscontrol/v2/models"
	"github.com/StackExchange/dnscontrol/v2/pkg/js"
	"github.com/StackExchange/dnscontrol/v2/pkg/normalize"
)

var _ = cmd(catDebug, func() *cli.Command {
	var args PrintIRArgs
	return &cli.Command{
		Name:  "print-ir",
		Usage: "Output intermediate representation (IR) after running validation and normalization logic.",
		Action: func(c *cli.Context) error {
			return exit(PrintIR(args))
		},
		Flags: args.flags(),
	}
}())

var _ = cmd(catDebug, func() *cli.Command {
	var args PrintIRArgs
	// This is the same as print-ir with the following changes:
	// - output defaults to /dev/null.
	// - prints "No errors." if there were no errors.
	return &cli.Command{
		Name:  "check",
		Usage: "Check and validate dnsconfig.js. Do not access providers.",
		Action: func(c *cli.Context) error {
			if args.Output == "" {
				args.Output = os.DevNull
			}
			err := exit(PrintIR(args))
			if err == nil {
				fmt.Fprintf(os.Stderr, "No errors.\n")
			}
			return err
		},
		Flags: args.flags(),
	}
}())

// PrintIRArgs encapsulates the flags/arguments for the print-ir command.
type PrintIRArgs struct {
	GetDNSConfigArgs
	PrintJSONArgs
	Raw bool
}

func (args *PrintIRArgs) flags() []cli.Flag {
	flags := append(args.GetDNSConfigArgs.flags(), args.PrintJSONArgs.flags()...)
	flags = append(flags, &cli.BoolFlag{
		Name:        "raw",
		Usage:       "Skip validation and normalization. Just print js result.",
		Destination: &args.Raw,
	})
	return flags
}

// PrintIR implements the print-ir subcommand.
func PrintIR(args PrintIRArgs) error {
	cfg, err := GetDNSConfig(args.GetDNSConfigArgs)
	if err != nil {
		return err
	}
	if !args.Raw {
		errs := normalize.NormalizeAndValidateConfig(cfg)
		if PrintValidationErrors(errs) {
			return fmt.Errorf("Exiting due to validation errors")
		}
	}
	return PrintJSON(args.PrintJSONArgs, cfg)
}

// PrintValidationErrors formats and prints the validation errors and warnings.
func PrintValidationErrors(errs []error) (fatal bool) {
	if len(errs) == 0 {
		return false
	}
	fmt.Printf("%d Validation errors:\n", len(errs))
	for _, err := range errs {
		if _, ok := err.(normalize.Warning); ok {
			fmt.Printf("WARNING: %s\n", err)
		} else {
			fatal = true
			fmt.Printf("ERROR: %s\n", err)
		}
	}
	return
}

// ExecuteDSL executes the dnsconfig.js contents.
func ExecuteDSL(args ExecuteDSLArgs) (*models.DNSConfig, error) {
	if args.JSFile == "" {
		return nil, fmt.Errorf("No config specified")
	}

	dnsConfig, err := js.ExecuteJavascript(args.JSFile, args.DevMode)
	if err != nil {
		return nil, fmt.Errorf("Executing javascript in %s: %s", args.JSFile, err)
	}
	return dnsConfig, nil
}

// PrintJSON outputs/prettyprints the IR data.
func PrintJSON(args PrintJSONArgs, config *models.DNSConfig) (err error) {
	var dat []byte
	if args.Pretty {
		dat, err = json.MarshalIndent(config, "", "  ")
	} else {
		dat, err = json.Marshal(config)
	}
	if err != nil {
		return err
	}
	if args.Output != "" {
		f, err := os.Create(args.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(dat)
		return err
	}
	fmt.Println(string(dat))
	return nil
}

func exit(err error) error {
	if err == nil {
		return nil
	}
	return cli.NewExitError(err, 1)
}
