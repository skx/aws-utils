// List the names of each cloudformation stack

package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type stacksCommand struct {

	// Path to a file containing roles
	rolesPath string

	// Show status too?
	status bool

	// Show all stacks, even deleted ones?
	all bool
}

// Arguments adds per-command args to the object.
func (sc *stacksCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&sc.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
	f.BoolVar(&sc.status, "status", false, "Show the stack-status as well as the name?")
	f.BoolVar(&sc.all, "all", false, "Show even deleted stacks?")
}

// Info returns the name of this subcommand.
func (sc *stacksCommand) Info() (string, string) {
	return "stacks", `List all cloudformation stack-names

Details:

This command allows you to list the names of all cloudformation stacks.

Listing stacks via the AWS CLI is otherwise a bit annoying, and here we
take care of excluding deleted stacks by default.  This makes it simpler
to use for scripting, and removes the necessity to have 'jq' available.
`

}

// Execute is invoked if the user specifies this sub-command.
func (sc *stacksCommand) Execute(args []string) int {

	//
	// Get the connection, using default credentials
	//
	session, err := utils.NewSession()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return 1
	}

	//
	// Now invoke our callback - this will call the function
	// "Search" once if we're not running with a role-file,
	// otherwise once for each role.
	//
	errs := utils.HandleRoles(session, sc.rolesPath, sc.DisplayStacks, nil)

	if len(errs) > 0 {
		fmt.Printf("errors running display\n")

		for _, err := range errs {
			fmt.Printf("%s\n", err)
		}
		return 1
	}

	return 0
}

// Display is our callback method, which is invoked once for our main
// account - if no roles-file is specified - or once for each assumed
// role within that file.
func (sc *stacksCommand) DisplayStacks(svc *ec2.EC2, account string, void interface{}) error {

	// Setup a session
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Get the cloudformation service
	cf := cloudformation.New(sess)
	input := &cloudformation.ListStacksInput{StackStatusFilter: []*string{}}

	// List the stacks
	resp, err := cf.ListStacks(input)
	if err != nil {
		return err
	}

	// Create a map for recording name => status.
	//
	// We do this because we want to show the stack-names in a
	// sorted-order.
	lookup := make(map[string]string)

	// Get all the stacks, and save their names/statuses in
	// a lookup table.
	for _, ent := range resp.StackSummaries {

		name := *ent.StackName
		status := *ent.StackStatus

		lookup[name] = status
	}

	// Sort the stack-names.
	keys := make([]string, 0, len(lookup))
	for key := range lookup {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Now we have a sorted list of stack-names we can iterate over them
	for _, key := range keys {

		// The stack-status comes from the lookup-map.
		val := lookup[key]

		// Skip deleted stacks, unless `-all` was specified.
		if !sc.all && strings.Contains(val, "DELETE") {
			continue
		}

		// Show the name of the stack
		fmt.Printf("%s", key)

		// If `-status` show the status too
		if sc.status {
			fmt.Printf(" %s", val)
		}

		// Newline
		fmt.Printf("\n")
	}

	return nil
}
