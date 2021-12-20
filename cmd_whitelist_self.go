// This is just useful!

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

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

	// Pattern contains the pattern to identify elements to update
	Name string

	// Role contains the AWS role to assume before making the change
	Role string

	// The port which will be whitelisted
	Port int
}

// Structure for our options and state.
type whitelistSelfCommand struct {

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
  * This MUST be unique.
* The port to open.
* Optionally you may specify the ARN of an AWS role to assume.

For example the following would be a good input file:

[
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] steve home",
        "Port": 443
    },
    {
        "SG": "sg-abcdef",
        "Name": "[aws-utils] steve home",
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
        "Name": "[aws-utils] steve home - https",
        "Port": 443
    },
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] steve home - ssh",
        "Port": 22
    }
]

NOTE: This only examines Ingress Rules, there are no changes made to Egress
rules.

To ease portability environmental variables are exported so you may write:

    "Name": "[aws-utils] - SSH - ${USER}",
`

}

// getIP returns the public IP address you're connecting from
func getIP() (string, error) {

	type IP struct {
		Query string
	}
	req, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		return "", err
	}
	defer req.Body.Close()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	var ip IP
	json.Unmarshal(body, &ip)

	return ip.Query + "/32", nil

}

// myIPDeleteCurrent removes the single rule within the specified
// security-group which has the description specified.
//
// If the description matches multiple rules then we abort, as we're
// only expecting one.
func myIPDeleteCurrent(svc *ec2.EC2, groupid, mdesc string, port int64) (bool, error) {

	// Get the contents of the group.
	current, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: aws.StringSlice([]string{groupid}),
	})
	if err != nil {
		return false, err
	}

	// Ensure that the name is non-unique before we do anything
	// destructive - count the number of rules that have the
	// specified text as the description.
	count := 0
	for _, sg := range current.SecurityGroups {
		for _, ipp := range sg.IpPermissions {
			for _, ipr := range ipp.IpRanges {
				if mdesc == *ipr.Description {
					count++
				}
			}
		}
	}

	// No match means we have nothing to remove, so we terminate early.
	if count == 0 {
		return false, nil
	}

	// If we have more than one we're going to abort.
	if count > 1 {
		return false, fmt.Errorf("there are %d rules which have the description '%s' - aborting the deletion", count, mdesc)
	}

	// For each security-group
	for _, sg := range current.SecurityGroups {

		// For each rule
		for _, ipp := range sg.IpPermissions {

			// for each CIDR range
			for _, ipr := range ipp.IpRanges {

				// Look for the description which is ours
				if mdesc == *ipr.Description {
					ipranges := []*ec2.IpRange{{
						CidrIp:      ipr.CidrIp,
						Description: aws.String(mdesc),
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
					if err != nil {
						return false, err
					}
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// myIPAdd adds a new CIDR range to the given security-group, with the
// specified port.
func myIPAdd(svc *ec2.EC2, groupid, mmyip, mdesc string, port int64) error {

	// Add the entry to the group
	var err error
	_, err = svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(groupid),
		IpPermissions: []*ec2.IpPermission{{
			FromPort:   aws.Int64(port),
			ToPort:     aws.Int64(port),
			IpProtocol: aws.String("tcp"),
			IpRanges: []*ec2.IpRange{{
				CidrIp:      aws.String(mmyip),
				Description: aws.String(mdesc),
			}},
		}},
	})

	return err
}

// Handle a change here
func handleSecurityGroup(entry ToChange, sess *session.Session, ip string) error {

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

	fmt.Printf("\n")
	fmt.Printf("  SecurityGroupID: %s\n", entry.SG)
	fmt.Printf("  IP:              %s\n", ip)
	fmt.Printf("  Port:            %d\n", entry.Port)
	fmt.Printf("  Description:     %s\n", entry.Name)
	if entry.Role != "" {
		fmt.Printf("  Role:            %s\n", entry.Role)
	}

	// Remove any existing rule with this name/description
	deleted, err := myIPDeleteCurrent(svc, entry.SG, entry.Name, int64(entry.Port))
	if err != nil {
		return err
	}

	// If we did make a change show that.
	if deleted {
		fmt.Printf("  Found existing entry named %s, and deleted it.\n", entry.Name)
	}

	err = myIPAdd(svc, entry.SG, ip, entry.Name, int64(entry.Port))
	if err != nil {
		return err
	}
	fmt.Printf("  Added new entry named %s, with current ip.\n", entry.Name)
	return nil
}

// Execute is invoked if the user chooses this sub-command.
func (i *whitelistSelfCommand) Execute(args []string) int {

	// Ensure we have a configuration file
	if len(args) < 1 {
		fmt.Printf("Usage: aws-utils whitelist-self config.json\n")
		return 1
	}

	// Read the file
	cnf, err := ioutil.ReadFile(args[0])
	if err != nil {
		fmt.Printf("Failed to read %s - %s\n", args[0], err)
		return 1
	}

	// All the entries we know we're going to change, as read from
	// the input JSON file.
	var changes []ToChange

	// Parse our JSON into a list of rules.
	if err = json.Unmarshal(cnf, &changes); err != nil {
		fmt.Printf("Error loading JSON: %s\n", err)
		return 1
	}

	// Get our remote IP.
	ip, err := getIP()
	if err != nil {
		fmt.Printf("Error finding your public IP: %s\n", err)
		return 1
	}
	fmt.Printf("Your remote IP is %s\n", ip)

	// Create a new AWS session
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		fmt.Printf("AWS login failed: %s\n", err.Error())
		return 1
	}

	// Process each group
	for _, entry := range changes {

		// Expand any variables in the name first
		entry.Name = os.ExpandEnv(entry.Name)

		// Now handle the additional/removal
		err := handleSecurityGroup(entry, sess, ip)
		if err != nil {
			fmt.Printf("error updating %s\n", err)
			return 1
		}
	}
	return 0
}
