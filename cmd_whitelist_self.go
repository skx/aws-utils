// This is just useful!

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/skx/aws-utils/utils"
	"github.com/skx/subcommands"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// ToChange contains the structure we're going to work with.
type ToChange struct {
	// SG contains the security-group ID which should be updated.
	SG string

	// Name contains the description of the rule that we'll update.
	// Updating here means "removing with old IP" and "adding with new IP".
	Name string

	// Role contains the AWS role to assume before proceeding to look
	// at the security-group specified.
	Role string

	// The port which will be whitelisted (TCP-only).
	Port int
}

// Structure for our options and state.
type whitelistSelfCommand struct {

	// IP contains the IP we've discovered.
	IP string

	// We embed the NoFlags option, because we accept no command-line flags.
	subcommands.NoFlags
}

// Info returns the name of this subcommand.
func (i *whitelistSelfCommand) Info() (string, string) {
	return "whitelist-self", `Update security-groups with your external IP.

Details:

Assume you have some security-groups which contain allow-lists of single IPs.
This command allows you to quickly and easily update those to keep your own
entry current.

You should provide a configuration file containing a series of rules, where
each rule contains:

* The security-group ID to which it applies.
* The description to use for the rule.
  * This MUST be unique within the security-group.
  * Duplicates will be detected and will stop processing.
* The port to open.
* Optionally you may specify the ARN of an AWS role to assume before starting.

For example the following would be a good input file:

[
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] Steve home",
        "Port": 443
    },
    {
        "SG": "sg-abcdef",
        "Name": "[aws-utils] Steve home",
        "Role": "arn:aws:iam::112233445566:role/devops-access-abcdef",
        "Port": 443
    }

]

When executed this command will then iterate over the rules contained in
the input-file.  For each rule it will examine the specified security-group,
removing any entry with the same name as you've specified, before re-adding
it with your current external IP.

While you may only specify a single port in a rule you can add multiple
rules to cover the case where you want to whitelist two ports - for example:

[
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] Steve home - HTTPS",
        "Port": 443
    },
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] Steve home - SSH",
        "Port": 22
    }
]

NOTE: This only examines Ingress Rules, there are no changes made to Egress
rules.

To ease portability environmental variables are exported so you may write:

    "Name": "[aws-utils] - SSH - ${USER}",
`

}

// getIP returns the public IP address of the user, via the use of
// the http://ip-api.com/ website.
func (i *whitelistSelfCommand) getIP() (string, error) {

	type IP struct {
		Query string
	}

	// Make a HTTP-request
	req, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		return "", err
	}
	defer req.Body.Close()

	// Read the body
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	// Decode the result
	var ip IP
	err = json.Unmarshal(body, &ip)
	if err != nil {
		return "", err
	}

	// Add a "/32" to the IP as that is what we'll need to use
	// when we're looking at the security-groups.
	return ip.Query + "/32", nil
}

// processSG looks at the security-group for any entry with the given
// description:
//
// 1.  If no entries exist with that description it is added.
//
// 2.  If multiple entries exist with that description report a fatal error.
//
// 3.  If a single entry exists with the wrong IP, remove it and add the new
//    IP.  Otherwise do nothing as the IP matches.
//
func (i *whitelistSelfCommand) processSG(svc *ec2.EC2, groupid, desc string, port int64) error {

	// Get the contents of the security group.
	current, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: aws.StringSlice([]string{groupid}),
	})
	if err != nil {
		return err
	}

	// Ensure that the description is unique before we do anything
	// destructive - count the number of rules that have the
	// specified text as the description.
	count := 0
	for _, sg := range current.SecurityGroups {
		for _, ipp := range sg.IpPermissions {
			for _, ipr := range ipp.IpRanges {
				if desc == *ipr.Description {
					count++
				}
			}
		}
	}

	// If we found zero rules which have the specified description
	// we need to add the new entry.
	if count == 0 {

		//
		// Add the current IP to the whitelist
		//
		return i.myIPAdd(svc, groupid, desc, port)
	}

	// If we have more than rule which contains the description then
	// we must abort.
	if count > 1 {
		return fmt.Errorf("there are %d rules which have the description '%s' - aborting", count, desc)
	}

	// OK we have one rule which has the expected description.
	//
	// Do we need to change the IP?
	for _, sg := range current.SecurityGroups {

		// For each rule
		for _, ipp := range sg.IpPermissions {

			// for each CIDR range
			for _, ipr := range ipp.IpRanges {

				// Look for the description which is ours
				if desc == *ipr.Description {

					// If the IP is the same
					// then we do nothing
					if *ipr.CidrIp == i.IP {
						fmt.Printf("  Existing entry already matches current IP - no change\n")
						return nil
					}

					fmt.Printf("  REMOVING %s from security-group.\n", *ipr.CidrIp)
					err = i.myIPDel(svc, groupid, desc, ipr, port)
					if err != nil {
						return fmt.Errorf("error removing entry %s", err)
					}

					return i.myIPAdd(svc, groupid, desc, port)
				}
			}
		}
	}
	return nil
}

// myIPDel removes a CIDR range from the given security-group, with the
// specified port.
func (i *whitelistSelfCommand) myIPDel(svc *ec2.EC2, groupid, desc string, ipr *ec2.IpRange, port int64) error {
	// Otherwise we need to delete
	// the existing rule, and add
	// a new one.
	ipranges := []*ec2.IpRange{{
		CidrIp:      ipr.CidrIp,
		Description: aws.String(desc),
	}}

	// Delete the rule we've found
	_, err := svc.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId: aws.String(groupid),
		IpPermissions: []*ec2.IpPermission{{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(port),
			ToPort:     aws.Int64(port),
			IpRanges:   ipranges,
		}},
	})
	return err
}

// myIPAdd adds a new CIDR range to the given security-group, with the
// specified port.
func (i *whitelistSelfCommand) myIPAdd(svc *ec2.EC2, groupid, desc string, port int64) error {

	fmt.Printf("  ADDING %s to security-group.\n", i.IP)
	// Add the entry to the group
	var err error
	_, err = svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(groupid),
		IpPermissions: []*ec2.IpPermission{{
			FromPort:   aws.Int64(port),
			ToPort:     aws.Int64(port),
			IpProtocol: aws.String("tcp"),
			IpRanges: []*ec2.IpRange{{
				CidrIp:      aws.String(i.IP),
				Description: aws.String(desc),
			}},
		}},
	})

	return err
}

// handleSecurityGroup handles the application of the rule to one
// security-group
func (i *whitelistSelfCommand) handleSecurityGroup(entry ToChange, sess *session.Session) error {

	// Get a handle to the service to use.
	svc := ec2.New(sess)

	// If we have a role then use it.
	if entry.Role != "" {
		// process
		creds := stscreds.NewCredentials(sess, entry.Role)

		// Create service client value configured for credentials
		// from assumed role.
		svc = ec2.New(sess, &aws.Config{Credentials: creds})
	}

	// No port specified?  Then default to HTTPS.
	if entry.Port == 0 {
		entry.Port = 443
	}
	if entry.Name == "" {
		fmt.Printf("%s  IGNORED rule with no Name field set.%s\n", colorRed, colorReset)
		return nil
	}

	fmt.Printf("\n")
	if entry.Role != "" {
		fmt.Printf("  Role:            %s\n", entry.Role)
	}
	fmt.Printf("  SecurityGroupID: %s\n", entry.SG)
	fmt.Printf("  IP:              %s\n", i.IP)
	fmt.Printf("  Port:            %d\n", entry.Port)
	fmt.Printf("  Description:     %s\n", entry.Name)

	// Remove any existing rule with this name/description
	err := i.processSG(svc, entry.SG, entry.Name, int64(entry.Port))
	if err != nil {
		return err
	}

	return nil
}

// processRules reads and processes rules contained within the specified
// JSON file.
func (i *whitelistSelfCommand) processRules(file string) error {

	// Read the file
	cnf, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s - %s", file, err)
	}

	// All the entries we know we're going to change, as read from
	// the input JSON file.
	var changes []ToChange

	// Parse our JSON into a list of rules.
	if err = json.Unmarshal(cnf, &changes); err != nil {
		return fmt.Errorf("error loading JSON: %s", err)
	}

	// Create a new AWS session
	sess, err := utils.NewSession()
	if err != nil {
		return fmt.Errorf("aws login failed: %s", err.Error())
	}

	// Process each group
	for _, entry := range changes {

		// Expand any variables in the name first
		entry.Name = os.ExpandEnv(entry.Name)

		// Now handle the additional/removal
		err := i.handleSecurityGroup(entry, sess)
		if err != nil {
			return fmt.Errorf("error updating %s", err)
		}
	}

	return nil
}

// Execute is invoked if the user chooses this sub-command.
func (i *whitelistSelfCommand) Execute(args []string) int {

	// Ensure we have a configuration file
	if len(args) < 1 {
		fmt.Printf("Usage: aws-utils whitelist-self config1.json config2.json .. configN.json\n")
		return 1
	}

	// Get our remote IP.
	ip, err := i.getIP()
	if err != nil {
		fmt.Printf("Error finding your public IP: %s\n", err)
		return 1
	}
	fmt.Printf("Your remote IP is %s\n", ip)

	// Save the current IP away
	i.IP = ip

	// For each filename on the command line
	for _, file := range args {

		// Process the file
		err = i.processRules(file)

		// Errors?  Then show them, but continue if there are more files
		if err != nil {
			fmt.Printf("Error %s\n", err)
		}
	}

	return 0
}
