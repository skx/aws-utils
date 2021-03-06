// List the names of each cloudformation stack

package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type stacksCommand struct {

	// filter allows filtering the returned stack-names
	filter string

	// Path to a file containing roles
	rolesPath string

	// policyPath is the policy-file to apply.
	policyPath string

	// policy is the contents of the policy file to apply
	policy string

	// Show status too?
	status bool

	// Show all stacks, even deleted ones?
	all bool
}

// Arguments adds per-command args to the object.
func (sc *stacksCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&sc.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
	f.StringVar(&sc.filter, "filter", "", "Show only stacks matching this filter")
	f.StringVar(&sc.policyPath, "policy", "", "Path to a stack-policy to apply to all stacks")
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

Once way to use this is to apply a stack policy to all stacks, that
can be done via the '-policy' argument.
`

}

// Execute is invoked if the user specifies this sub-command.
func (sc *stacksCommand) Execute(args []string) int {

	//
	// If we have a policy-path then read it into our policy
	//
	if sc.policyPath != "" {
		content, err := os.ReadFile(sc.policyPath)
		if err != nil {
			fmt.Printf("error reading %s: %s\n", sc.policyPath, err)
			return 1
		}

		sc.policy = string(content)
	}

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
	// "DisplayStacks" once if we're not running with a role-file,
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

	// Create a map for recording name => [status1, status2..].
	//
	// We do this primarily because we want to show the
	// stack-names in sorted-order.
	lookup := make(map[string][]string)

	// Get all the stacks, and save their names/statuses in
	// a lookup table.
	for _, ent := range resp.StackSummaries {

		// Get the nam/status
		name := *ent.StackName
		status := *ent.StackStatus

		// Append the status to the name.
		//
		// This is necessary because the same stack-name might
		// be present multiple times, in different states:
		//
		//  [DELETE_COMPLETE, DELETE_COMPLETE, UPDATE_COMPLETE]
		//
		lookup[name] = append(lookup[name], status)
	}

	// Sort the stack-names.
	keys := make([]string, 0, len(lookup))
	for key := range lookup {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Now we have a sorted list of stack-names we can iterate over them
	for _, key := range keys {

		// The stack-statuses comes from the lookup-map.
		val := lookup[key]

		// A stack might appear multiple times, in different
		// states:
		//
		// DELETE_COMPLETE, DELETE_COMPLETE, UPDATE_COMPLETE
		show := false

		// Don't show if "DELETE_COMPLETE"
		for _, state := range val {
			if !strings.Contains(state, "DELETE") {
				show = true
			}
		}

		// Should we filter this stack out?
		if sc.filter != "" {
			// If it doesn't match then skip it.
			match, er := regexp.MatchString(sc.filter, key)
			if er != nil {
				return fmt.Errorf("error running regexp match of %s against %s: %s", sc.filter, key, er)
			}
			if !match {
				show = false
			}
		}

		// Are we showing all?
		if !sc.all && !show {
			continue
		}

		// Show the name of the stack
		fmt.Printf("%s", key)

		// If `-status` show the status too
		if sc.status {
			fmt.Printf(" [%s]", strings.Join(val, ","))
		}

		// Applying a policy?
		if sc.policy != "" {

			// Create the parameters
			params := &cloudformation.SetStackPolicyInput{
				StackName:       aws.String(key),
				StackPolicyBody: aws.String(sc.policy),
			}

			// Set the policy
			resp, err := cf.SetStackPolicy(params)
			if err != nil {
				fmt.Printf("error calling SetStackPolicy %s\n", err)
				return err
			}

			// Show the response
			fmt.Printf("SetStackPolicy(%s) -> %s\n", key, resp)
		}

		// Newline
		fmt.Printf("\n")
	}

	return nil
}
