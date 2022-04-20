// Show summary of instance details, as CSV.
//
// Primarily written to find instances running with old AMIs

package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/skx/aws-utils/instances"
	"github.com/skx/aws-utils/tag2name"
	"github.com/skx/aws-utils/utils"

	"github.com/aws/aws-sdk-go/aws/awserr"
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

Valid fields are

* "account" - The AWS account-number.
* "az" - The availability zone within which the instance is running.
* "ami" - The AMI name of the running instance.
* "amiage" - The age of the AMI in days.
* "id" - The instance ID.
* "name" - The instance name, as set via tags.
* "privateipv4" - The (private) IPv4 address associated with the instance.
* "publicipv4" - The (public) IPv4 address associated with the instance.
* "ssh-key" - The SSH key setup for this instance.
* "state" - The instance state (running, pending, etc).
* "subnet" - The name of the subnet within which the instance is running.
* "subnetid" - The ID of the subnet within which the instance is running.
* "type" - The instance type (t2.small, t3.large, etc).
* "vpc" - The name of the VPC within which the instance is running.
* "vpcid" - The ID of the VPC within which the instance is running.
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

	// Map of subnet names to IDs.
	var subnets map[string]string
	fetchSubnets := false

	// Map of VPC names to IDs
	var vpcs map[string]string
	fetchVPCs := false

	// Split the fields, by comma
	supplied := strings.Split(format, ",")

	// Ensure all fields are lower-cased and stripped of spaces
	fields := []string{}
	for _, field := range supplied {
		field = strings.TrimSpace(field)
		field = strings.ToLower(field)
		fields = append(fields, field)

		// Do we need to fetch subnet information?
		if field == "subnet" {
			fetchSubnets = true
		}
		if field == "vpc" {
			fetchVPCs = true
		}
	}

	// Fetch the subnets within the account, if we're going
	// to display the human-readable name.
	if fetchSubnets {

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
			return fmt.Errorf("failed to get subnets for account %s", acct)
		}

		// fill up our map
		subnets = make(map[string]string)

		// populate it with "id -> name"
		for i := range result.Subnets {
			// Get the name, via tags, if present
			name := tag2name.Lookup(result.Subnets[i].Tags, "unnamed")
			subnets[*result.Subnets[i].SubnetId] = name
		}
	}

	// Fetch the VPCs within the account, if we're going
	// to display the human-readable name.
	if fetchVPCs {

		// An empty filter, to get all subnets
		input := &ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
				{},
			},
		}

		// describe the vpcs
		result, err := svc.DescribeVpcs(input)
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
			return fmt.Errorf("failed to get VPCs for account %s", acct)
		}

		// fill up our map
		vpcs = make(map[string]string)

		// populate it with "id -> name"
		for i := range result.Vpcs {
			// Get the name, via tags, if present
			name := tag2name.Lookup(result.Vpcs[i].Tags, "unnamed")
			vpcs[*result.Vpcs[i].VpcId] = name
		}
	}

	// For each instance we've discovered
	for _, obj := range ret {

		// If we've not printed the header..
		if !c.header {

			// Show something human-readable
			for i, field := range fields {

				switch field {
				case "account":
					fmt.Printf("Account ID")
				case "ami":
					fmt.Printf("AMI ID")
				case "amiage":
					fmt.Printf("AMI Age")
				case "az":
					fmt.Printf("Availability Zone")
				case "id":
					fmt.Printf("Instance ID")
				case "name":
					fmt.Printf("Name")
				case "privateipv4":
					fmt.Printf("PrivateIPv4")
				case "publicipv4":
					fmt.Printf("PublicIPv4")
				case "ssh-key":
					fmt.Printf("SSH Key")
				case "state":
					fmt.Printf("Instance State")
				case "subnet":
					fmt.Printf("Subnet")
				case "subnetid":
					fmt.Printf("Subnet ID")
				case "type":
					fmt.Printf("Instance Type")
				case "vpc":
					fmt.Printf("VPC")
				case "vpcid":
					fmt.Printf("VPC ID")
				default:
					fmt.Printf("unknown field:%s", field)
				}

				// if this isn't the last one, add ","
				if i < len(fields)-1 {
					fmt.Printf(",")
				}

			}

			// Terminate the header with a newline
			fmt.Printf("\n")
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
			case "az":
				fmt.Printf("%s", obj.AvailabilityZone)
			case "id":
				fmt.Printf("%s", obj.InstanceID)
			case "name":
				fmt.Printf("%s", obj.InstanceName)
			case "privateipv4":
				fmt.Printf("%s", obj.PrivateIPv4)
			case "publicipv4":
				fmt.Printf("%s", obj.PublicIPv4)
			case "ssh-key":
				fmt.Printf("%s", obj.SSHKeyName)
			case "state":
				fmt.Printf("%s", obj.InstanceState)
			case "subnet":
				fmt.Printf("%s", subnets[obj.SubnetID])
			case "subnetid":
				fmt.Printf("%s", obj.SubnetID)
			case "type":
				fmt.Printf("%s", obj.InstanceType)
			case "vpc":
				fmt.Printf("%s", vpcs[obj.VPCID])
			case "vpcid":
				fmt.Printf("%s", obj.VPCID)
			default:
				fmt.Printf("unknown field:%s", field)
			}

			// if this isn't the last one, add ","
			if i < len(fields)-1 {
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
