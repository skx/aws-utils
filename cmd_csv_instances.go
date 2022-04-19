// Show summary of instance details, as CSV.
//
// Primarily written to find instances running with old AMIs

package main

import (
	"flag"
	"fmt"
	"strings"

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

	// Format string to print
	format string
}

// Arguments adds per-command args to the object.
func (c *csvInstancesCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&c.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
	f.StringVar(&c.format, "format", "", "Format string of the fields to print")
}

// Info returns the name of this subcommand.
func (c *csvInstancesCommand) Info() (string, string) {
	return "csv-instances", `Export a summary of running instances.

Details:

This command exports a list of the running instances which are available
to the logged in account, in CSV format.

By default the export contains the following fields:

* Account ID
* Instance ID
* Instance Name
* AMI ID

You can specify a different output via the 'format' argument, for
example:

     aws-utils csv-instances --format="account,id,name,ipv4address"
`

}

// DumpCSV outputs the list of running instances.
func (c *csvInstancesCommand) DumpCSV(svc *ec2.EC2, acct string, void interface{}) error {

	// Get the running instances.
	ret, err := instances.GetInstances(svc, acct)
	if err != nil {
		return err
	}

	// Get the format-string
	format := c.format
	if format == "" {
		format = "account,id,name,ami"
	}

	// Split the fields, by comma
	fields := strings.Split(format, ",")

	// For each one we've found
	for _, obj := range ret {

		// If we've not printed the header..
		if !c.header {

			// Show something human-readable
			for i, field := range fields {

				switch field {
				case "account":
					fmt.Printf("Account")
				case "ami":
					fmt.Printf("AMI")
				case "amiage":
					fmt.Printf("AMI Age")
				case "id":
					fmt.Printf("Instance ID")
				case "privateipv4":
					fmt.Printf("PrivateIPv4")
				case "publicipv4":
					fmt.Printf("PublicIPv4")
				case "ssh-key":
					fmt.Printf("SSH Key")
				case "state":
					fmt.Printf("Instance State")
				case "type":
					fmt.Printf("Instance Type")
				case "name":
					fmt.Printf("Name")
				default:
					fmt.Printf("unknown field:%s", field)
				}

				// if this isn't the last one, add ","
				if i < len(fields) {
					fmt.Printf(",")
				}

			}
			c.header = true
		}

		// Show each field
		for i, field := range fields {

			switch field {
			case "account":
				fmt.Printf("%s", acct)
			case "ami":
				fmt.Printf("%s", obj.InstanceAMI)
			case "amiage":
				fmt.Printf("%d", obj.AMIAge)
			case "id":
				fmt.Printf("%s", obj.InstanceID)
			case "privateipv4":
				fmt.Printf("%s", obj.PrivateIPv4)
			case "publicipv4":
				fmt.Printf("%s", obj.PublicIPv4)
			case "ssh-key":
				fmt.Printf("%s", obj.SSHKeyName)
			case "state":
				fmt.Printf("%s", obj.InstanceState)
			case "type":
				fmt.Printf("%s", obj.InstanceType)
			case "name":
				fmt.Printf("%s", obj.InstanceName)
			default:
				fmt.Printf("unknown field:%s", field)
			}

			// if this isn't the last one, add ","
			if i < len(fields) {
				fmt.Printf(",")
			}
		}

		// Newline between records
		fmt.Printf("\n")

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
