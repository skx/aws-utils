// Show orphaned Route53 domains.

package main

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/skx/aws-utils/utils"
	"github.com/skx/subcommands"
)

// Structure for our options and state.
type orphanedZonesCommand struct {

	// We embed the NoFlags option, because we accept no command-line flags.
	subcommands.NoFlags
}

// Info returns the name of this subcommand.
func (i *orphanedZonesCommand) Info() (string, string) {
	return "orphaned-zones", `Show orphaned Route53 zones.

Details:

This command retrieves a list of domains hosted on AWS Route53, and
reports those which are orphaned.

An orphaned domain is one which has all NS records pointing outside the
AWS system.  (Specifically we look for a NS record which does not contain
the substring "aws" in its hostname.)
`

}

// Execute is invoked if the user specifies this sub-command.
func (i *orphanedZonesCommand) Execute(args []string) int {

	// Start a session
	sess, err := utils.NewSession()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return 1
	}

	// Get the service handle
	svc := route53.New(sess)

	// Get all the results
	r, err := svc.ListHostedZones(&route53.ListHostedZonesInput{})
	if err != nil {
		fmt.Printf("failed to call ListHostedZones: %s\n", err)
		return 1
	}

	// Collect orphans here so that we can display them neatly at the
	// end of our output.
	orphaned := []string{}

	// Process each domain
	for _, entry := range r.HostedZones {

		// Lookup the nameservers
		nameserver, err := net.LookupNS(*entry.Name)
		if err != nil {
			fmt.Printf("Failed to lookup NS record for %s: %s\n", *entry.Name, err)
		}

		// Assume hosted in AWS, so valid and not orphaned
		valid := true

		// Look at the nameservers, if they don't contain "aws"
		// then we've got an orphan
		for _, ns := range nameserver {
			n := fmt.Sprintf("%s", ns)
			if !strings.Contains(n, "aws") {
				valid = false
			}
		}

		// If valid then show it.
		if valid {
			fmt.Printf("VALID  - %s\n", *entry.Name)
		} else {

			// Save it in our orphan-list
			orphaned = append(orphaned, *entry.Name)
		}

	}

	// Show orphaned records before we terminate.
	sort.Strings(orphaned)
	for _, entry := range orphaned {
		fmt.Printf("ORPHAN - %s\n", entry)
	}

	return 0
}
