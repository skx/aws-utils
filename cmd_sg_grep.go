// Search security-groups.
//
// Primarily written as an everyday useful tool.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
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

	failed := false

	if len(args) < 1 {
		fmt.Printf("Usage: aws-utils sg-grep term1 ..\n")
		return 1
	}

	//
	// Get the connection, using default credentials
	//
	sess, err2 := utils.NewSession()
	if err2 != nil {
		fmt.Printf("%s\n", err2.Error())
		return 1
	}

	//
	// Create a new session to find our account
	//
	stsSvc := sts.New(sess)
	out, err3 := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err3 != nil {
		fmt.Printf("Failed to get identity: %s", err3.Error())
		return 1
	}

	//
	// This is our account ID
	//
	acct := *out.Account

	//
	// If we have no role-list then just dump our current account
	//
	if sg.rolesPath == "" {

		svc := ec2.New(sess)

		// For each term
		for _, term := range args {

			// Run the search
			err := sg.search(svc, acct, term)
			if err != nil {
				fmt.Printf("%sError: %s%s\n", colorRed, err, colorReset)
				return 1
			}
		}
		return 0
	}

	//
	// OK we have a list of roles, handle them one by one
	//
	file, err := os.Open(sg.rolesPath)
	if err != nil {
		fmt.Printf("Error opening role-file: %s %s\n", sg.rolesPath, err.Error())
		return 1
	}
	defer file.Close()

	//
	// Process the role-file line by line
	//
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		// Get the line
		role := scanner.Text()

		// Skip comments
		if strings.HasPrefix(role, "#") {
			continue
		}

		// process
		creds := stscreds.NewCredentials(sess, role)

		// Create service client value configured for credentials
		// from assumed role.
		svc := ec2.New(sess, &aws.Config{Credentials: creds})

		// for each term
		for _, term := range args {

			// Run the search
			err := sg.search(svc, role, term)
			if err != nil {
				fmt.Printf("%sError: %s%s\n", colorRed, err, colorReset)
				failed = true
			}
		}
	}

	//
	// Error processing the end of the file?
	//
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error processing role-file: %s %s\n", sg.rolesPath, err.Error())
		return 1
	}

	if failed {
		return 1
	}
	return 0
}

//
// Perform a search of the security-groups in the given region for
// the specified text.
//
func (sg *sgGrepCommand) search(svc *ec2.EC2, account string, term string) error {

	// Compile the term into a regular expression
	//
	// NOTE: We add the `(?i)` prefix, to make this case insensitive.
	r, err := regexp.Compile("(?i)" + term)
	if err != nil {
		return fmt.Errorf("unable to compile regexp %s - %s", term, err)

	}

	// Retrieve the security group descriptions
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
