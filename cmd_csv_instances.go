// Show summary of instance details, as CSV.
//
// Primarily written to find instances running with old AMIs

package main

import (
	"flag"
	"fmt"

	"github.com/skx/aws-utils/instances"
	"github.com/skx/aws-utils/utils"

	"github.com/aws/aws-sdk-go/service/ec2"
)

// Structure for our options and state.
type csvInstancesCommand struct {

	// Path to a file containing roles
	rolesPath string

	// Have we shown the CSV header?
	header bool
}

// Arguments adds per-command args to the object.
func (c *csvInstancesCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&c.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
}

// Info returns the name of this subcommand.
func (c *csvInstancesCommand) Info() (string, string) {
	return "csv-instances", `Export a summary of running instances.

Details:

This command exports a list of the running instances which are available
to the logged in account, in CSV format.

The export contains:

* Account ID
* Instance ID
* Instance Name
* AMI ID
* Age of AMI in days

Other fields might be added in the future.
`

}

// Sync from remote to local
func (c *csvInstancesCommand) DumpCSV(svc *ec2.EC2, acct string, void interface{}) error {

	ret, err := instances.GetInstances(svc, acct)

	if err != nil {
		return err
	}

	for _, obj := range ret {
		if !c.header {
			fmt.Printf("Account, Instance ID, Name, AMI, AMI Age\n")
			c.header = true
		}

		//
		// Now show all the information in CSV format
		//
		fmt.Printf("%s,%s,%s,%s,%d\n", acct, obj.InstanceID, obj.InstanceName, obj.InstanceAMI, obj.AMIAge)

	}
	return nil
}

// Execute is invoked if the user specifies this subcommand.
func (c *csvInstancesCommand) Execute(args []string) int {

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
	// "DumpCSV" once if we're not running with a role-file,
	// otherwise once for each role.
	//
	errs := utils.HandleRoles(session, c.rolesPath, c.DumpCSV, nil)
	if len(errs) > 0 {
		fmt.Printf("errors running CSV-Dump\n")
		for _, err := range errs {
			fmt.Printf("%s\n", err)
		}
		return 1
	}

	return 0
}
