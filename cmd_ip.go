// Show private IPv4 address of the named instance.
//
// Primarily written to handle tab-completion

package main

import (
	"fmt"
	"flag"
	"regexp"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/instances"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type ipCommand struct {

	// Are we verbose?
	verbose bool
}


// Arguments adds per-command args to the object.
func (i *ipCommand) Arguments(f *flag.FlagSet) {
	f.BoolVar(&i.verbose, "verbose", false, "Should we show the matching name too?")
}

// Info returns the name of this subcommand.
func (i *ipCommand) Info() (string, string) {
	return "ip", `Show the private IP of the given instance.

Details:

This command simply outputs the first private IP address of the instance
which matches the given regular expression.

    $ aws-utils ip *prod*manager
    10.12.43.120

Unlike other commands this explicitly does not support the use of a role-path,
being limited to the account signed in, and any assumed role only.

It is useful for command-line completion, and similar scripting purposes.`

}

// DumpInstances looks up the appropriate details and outputs them to the
// console, via the use of a provided template.
func (i *ipCommand) OutputInformation(svc *ec2.EC2, acct string, void interface{}) error {

	// Get the name we're completing upon
	name := void.(string)

	// Get the instances that are running.
	ret, err := instances.GetInstances(svc, acct)
	if err != nil {
		return err
	}

	// For each one, output the appropriate thing.
	for _, obj := range ret {

		// match against the name
		m, err := regexp.MatchString(name,obj.InstanceName)
		if err != nil {
			return fmt.Errorf("error running regexp match %s", err)
		}

		// if there was a match
		if m {
			if ( i.verbose ) {
				// show IP + name if being verbose
				fmt.Printf("%s %s\n", obj.PrivateIPv4, obj.InstanceName)
			} else {
				// otherwise just the IP.
				fmt.Printf("%s\n", obj.PrivateIPv4)
			}
		}
	}
	return nil
}

// Execute is invoked if the user specifies this subcommand.
func (i *ipCommand) Execute(args []string) int {

	//
	// Get the connection, using default credentials
	//
	session, err := utils.NewSession()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return 1
	}

	for _, name := range args {

		//
		// Now invoke our callback which allows iteration over
		// available instances.
		//
		// Pass the name, but don't pass a role-path.
		//
		errs := utils.HandleRoles(session, "", i.OutputInformation, name)

		if len(errs) > 0 {
			fmt.Printf("errors encountered running this operation:\n")
			for _, err := range errs {
				fmt.Printf("%s\n", err)
			}
			return 1
		}
	}

	return 0
}
