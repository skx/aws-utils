// Search security-groups.
//
// Primarily written as an everyday useful tool.

package main

import (
	"flag"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type sgGrepCommand struct {

	// Path to a file containing roles
	rolesPath string
}

// Arguments adds per-command args to the object.
func (sg *sgGrepCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&sg.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
}

// Info returns the name of this subcommand.
func (sg *sgGrepCommand) Info() (string, string) {
	return "sg-grep", `Security-Group Grep

Details:

This command allows you to run grep against security-groups.
`

}

// Execute is invoked if the user specifies this sub-command.
func (sg *sgGrepCommand) Execute(args []string) int {

	if len(args) < 1 {
		fmt.Printf("Usage: aws-utils sg-grep term1 ..\n")
		return 1
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
	// "Search" once if we're not running with a role-file,
	// otherwise once for each role.
	//
	errs := utils.HandleRoles(session, sg.rolesPath, sg.Search, args)

	if len(errs) > 0 {
		fmt.Printf("errors running search\n")

		for _, err := range errs {
			fmt.Printf("%s\n", err)
		}
		return 1
	}

	return 0
}

// Search is our callback method, which is invoked once for our main
// account - if no roles-file is specified - or once for each assumed
// role within that file.
//
// We return our search-terms to their array-form, and perform a single
// search for each one.
func (sg *sgGrepCommand) Search(svc *ec2.EC2, account string, void interface{}) error {

	// Get our search-terms back as an array
	terms := void.([]string)

	// For each one ..
	for _, term := range terms {

		// Run the search
		err := sg.searchTerm(svc, account, term)

		// return any error
		if err != nil {
			return err
		}
	}
	return nil
}

// searchTerm runs the search within one AWS account, and reports upon
// any matches
func (sg *sgGrepCommand) searchTerm(svc *ec2.EC2, account string, term string) error {

	// Compile the term into a regular expression
	//
	// NOTE: We add the `(?i)` prefix, to make this case insensitive.
	r, err := regexp.Compile("(?i)" + term)
	if err != nil {
		return fmt.Errorf("unable to compile regexp %s - %s", term, err)

	}

	// Retrieve the security groups
	result, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return fmt.Errorf("unable to get security-groups %s", err)
	}

	// For each security-group we find.
	for _, group := range result.SecurityGroups {

		// Get the contents as a string.
		txt := group.String()

		// If the string matches our regular expression we're good.
		if r.MatchString(txt) {

			// Show ID + description
			fmt.Printf("AWS Account:%s %s - %s\n", account, *group.GroupId, *group.Description)

			// Show contents of the SG - with a leading TAB
			lines := strings.Split(txt, "\n")
			for _, line := range lines {
				fmt.Printf("\t%s\n", line)
			}

		}
	}

	return nil
}
