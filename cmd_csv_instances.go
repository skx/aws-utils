// Show summary of instance details, as CSV.
//
// Primarily written to find instances running with old AMIs

package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/skx/aws-utils/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Cache of creation-time/date
var cache map[string]string

// Structure for our options and state.
type csvInstancesCommand struct {

	// Path to a file containing roles
	rolesPath string
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

// Get the creation-date of the given AMI.
//
// Values are cached.
func (c *csvInstancesCommand) amiCreation(svc *ec2.EC2, id string) (string, error) {

	// Lookup in the cache to see if we've already found the creation
	// date for this AMI
	cached, ok := cache[id]
	if ok {
		return cached, nil
	}

	// Setup a filter for the AMI we're looking for.
	input := &ec2.DescribeImagesInput{
		ImageIds: []*string{
			aws.String(id),
		},
	}

	// Run the search
	result, err := svc.DescribeImages(input)
	if err != nil {
		// Message from an error.
		return "", fmt.Errorf("error getting image info: %s", err.Error())
	}

	// If we got a result then we can return the creation time
	// (as a string)
	if len(result.Images) > 0 {

		// But save in a cache for the future
		date := *result.Images[0].CreationDate
		cache[id] = date
		return date, nil
	}
	return "", fmt.Errorf("no date for %s", id)
}

// Sync from remote to local
func (c *csvInstancesCommand) DumpCSV(svc *ec2.EC2, acct string, void interface{}) error {

	// Get the instances which are running/pending
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running"), aws.String("pending")},
			},
		},
	}

	// Create new EC2 client
	result, err := svc.DescribeInstances(params)
	if err != nil {
		return fmt.Errorf("DescribeInstances failed: %s", err)
	}

	// For each instance show stuff
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {

			// We have a running EC2 instnace.

			// Collect the data we want
			id := *instance.InstanceId

			// Find the name.
			name := *instance.InstanceId

			// Look for the name, which is set via a Tag.
			i := 0
			for i < len(instance.Tags) {

				if *instance.Tags[i].Key == "Name" {
					name = *instance.Tags[i].Value
				}
				i++
			}

			// AMI name
			ami := *instance.ImageId

			//
			// Get the AMI creation-date
			//
			create, err := c.amiCreation(svc, ami)
			if err != nil {
				return fmt.Errorf("failed to get creation date of %s: %s", ami, err.Error())
			}

			//
			// Parse the date, so we can report how many days
			// ago the AMI was created.
			//
			t, err := time.Parse("2006-01-02T15:04:05.000Z", create)
			if err != nil {
				return fmt.Errorf("failed to parse time string %s: %s", create, err)
			}

			//
			// Count how old the AMI is in days
			//
			date := time.Now()
			diff := date.Sub(t)
			create = fmt.Sprintf("%d", (int(diff.Hours() / 24)))

			//
			// Now show all the information in CSV format
			//
			fmt.Printf("%s,%s,%s,%s,%s\n", acct, id, name, ami, create)

		}
	}
	return nil
}

// Execute is invoked if the user specifies this subcommand.
func (c *csvInstancesCommand) Execute(args []string) int {

	//
	// Create our cache
	//
	cache = make(map[string]string)

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
