// List the subnets available within the various accounts.

package main

import (
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type subnetsCommand struct {

	// Path to a file containing roles
	rolesPath string

	// show the header already?
	header bool
}

// Arguments adds per-command args to the object.
func (sc *subnetsCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&sc.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
}

// Info returns the name of this subcommand.
func (sc *subnetsCommand) Info() (string, string) {
	return "subnets", `List all subnets, and their names.

Details:

This command allows you to list the names of all subnets, and their
associated CIDR ranges.  All available VPCs will be exported in a
simple CSV format, complete with header.
`

}

// Execute is invoked if the user specifies this sub-command.
func (sc *subnetsCommand) Execute(args []string) int {

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
	errs := utils.HandleRoles(session, sc.rolesPath, sc.DisplaySubnets, nil)

	if len(errs) > 0 {
		fmt.Printf("errors running display\n")

		for _, err := range errs {
			fmt.Printf("%s\n", err)
		}
		return 1
	}

	return 0
}

// DisplaySubnets is our callback method, which is invoked once for our main
// account - if no roles-file is specified - or once for each assumed
// role within that file.
func (sc *subnetsCommand) DisplaySubnets(svc *ec2.EC2, account string, void interface{}) error {

	// An empty filter, to get all subnets
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{},
		},
	}

	// describe the subnets
	result, err := svc.DescribeSubnets(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return fmt.Errorf("failed to get subnets for account %s", account)
	}

	// For each subnet
	for i := range result.Subnets {

		// Get the name, via tags, if present
		name := "unnamed"
		n := 0
		for _, tag := range result.Subnets[i].Tags {

			if *tag.Key == "Name" {
				name = *tag.Value
			}
			n++
		}

		// Show the details
		if !sc.header {
			fmt.Printf("Account, VPC, Subnet Name, Subnet ID, Cidr\n")
			sc.header = true
		}
		fmt.Printf("%s,%s,%s,%s,%s\n", account, *result.Subnets[i].VpcId, name, *result.Subnets[i].SubnetId, *result.Subnets[i].CidrBlock)
	}

	return nil
}
