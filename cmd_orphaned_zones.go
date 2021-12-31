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

	// Collect orphans, errors, and valid domains in these
	// arrays so we can show them (sorted) at the end of our
	// processing
	valid := []string{}
	error := []string{}
	orphan := []string{}

	// Process each domain
	for _, entry := range r.HostedZones {

		// Lookup the nameservers, if there's an error skip
		nameserver, err := net.LookupNS(*entry.Name)
		if err != nil {
			error = append(error, fmt.Sprintf("%s - %s", *entry.Name, err))
			continue
		}

		// Now we have the nameserver(s) look for ones that
		// contain the string "aws".  This is a proxy for being
		// hosted by route53 (still).
		aws := true
		for _, ns := range nameserver {
			n := fmt.Sprintf("%s", ns)
			if !strings.Contains(n, "aws") {
				aws = false
			}
		}
		if aws {
			valid = append(valid, *entry.Name)
		} else {
			orphan = append(orphan, *entry.Name)
		}
	}

	// show results: valid, orphaned, error
	sort.Strings(valid)
	for _, entry := range valid {
		fmt.Printf("VALID  - %s\n", entry)
	}
	sort.Strings(orphan)
	for _, entry := range orphan {
		fmt.Printf("ORPHAN - %s\n", entry)
	}
	sort.Strings(error)
	for _, entry := range error {
		fmt.Printf("ERROR  - %s\n", entry)
	}

	return 0
}
