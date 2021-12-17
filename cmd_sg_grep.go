// Search security-groups.
//
// Primarily written as an everyday useful tool.

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/skx/subcommands"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Structure for our options and state.
type sgGrepCommand struct {

	// We embed the NoFlags option, because we accept no command-line flags.
	subcommands.NoFlags
}

// Info returns the name of this subcommand.
func (i *sgGrepCommand) Info() (string, string) {
	return "sg-grep", `Security-Group Grep

Details:

This command allows you to run grep against security-groups.
`

}

// Execute is invoked if the user specifies this sub-command.
func (i *sgGrepCommand) Execute(args []string) int {

	if len(args) < 1 {
		fmt.Printf("Usage: aws-utils sg-grep term1 ..\n")
		return 1
	}

	for _, term := range args {
		search(term)
	}

	return 0
}

//
// Perform a search of the security-groups in the given region for
// the specified text.
//
func search(term string) {

	// Compile the term into a regular expression
	//
	// NOTE: We add the `(?i)` prefix, to make this case insensitive.
	r, err := regexp.Compile("(?i)" + term)
	if err != nil {
		fmt.Printf("Error compiling regular expression %s - %s\n", term, err)
		os.Exit(1)
	}

	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		fmt.Printf("Error creating session %s\n", err)
		os.Exit(1)
	}

	// Create an EC2 service client.
	svc := ec2.New(sess)

	// Retrieve the security group descriptions
	result, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidGroupId.Malformed":
				fallthrough
			case "InvalidGroup.NotFound":
				fmt.Printf("Error:%s.", aerr.Message())
				os.Exit(1)
			}
		}
		fmt.Printf("Error:Unable to get descriptions for security groups, %v", err)
		os.Exit(1)
	}

	// For each security-group we find.
	for _, group := range result.SecurityGroups {

		// Get the contents as a string.
		txt := group.String()

		// If the string matches our regular expression we're good.
		if r.MatchString(txt) {

			// Show ID + description
			fmt.Printf("%s - %s\n", *group.GroupId, *group.Description)

			// Show contents of the SG - with a leading TAB
			lines := strings.Split(txt, "\n")
			for _, line := range lines {
				fmt.Printf("\t%s\n", line)
			}

		}
	}
}
