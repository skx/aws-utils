// This is just useful!

package main

import (
	"encoding/json"
	"fmt"
	"github.com/skx/subcommands"
	"io/ioutil"
	"net/http"

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
type remoteIPChangeCommand struct {

	// We embed the NoFlags option, because we accept no command-line flags.
	subcommands.NoFlags
}

// Info returns the name of this subcommand.
func (i *remoteIPChangeCommand) Info() (string, string) {
	return "remote-ip-change", `Update security-groups with your external IP.

Details:

Assume you have some security-groups which contain allow-lists of single IPs.
This command allows you to quickly and easily update those.

You should provide a configuration file containing:

* The security-group IDs
* The description to use for the rule.
* The port to open in the security-group rule.
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
the specified security-group, and remove any existing rule with the same name,
before adding a new rule with your current IP.

NOTE: This only examines Ingress Rules, there are no changes made to Egress
rules.
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

func myIPDeleteCurrent(svc *ec2.EC2, groupid, mmyip, mdesc string) (bool, error) {

	// Get the contents of the group.
	current, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: aws.StringSlice([]string{groupid}),
	})
	if err != nil {
		return false, err
	}

	// For each security-group
	for _, sg := range current.SecurityGroups {

		// For each rule
		for _, ipp := range sg.IpPermissions {
			for _, ipr := range ipp.IpRanges {

				flag := false
				ipranges := []*ec2.IpRange{}

				// Look for a rule that is "ours"
				if mmyip == *ipr.CidrIp {
					flag = true
					ipranges = []*ec2.IpRange{{
						CidrIp: aws.String(mmyip),
					}}
				}
				// Look for the description which is ours
				if mdesc == *ipr.Description {
					flag = true
					ipranges = []*ec2.IpRange{{
						CidrIp:      ipr.CidrIp,
						Description: aws.String(mdesc),
					}}
				}
				// Not found the IP/description?
				if !flag {
					continue
				}
				// Delete the rule
				_, err := svc.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
					GroupId: aws.String(groupid),
					IpPermissions: []*ec2.IpPermission{{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int64(443),
						ToPort:     aws.Int64(443),
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
	return false, nil
}

func myIPAdd(svc *ec2.EC2, groupid, mmyip, mdesc string) error {

	// Add the entry to the group
	var err error
	_, err = svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(groupid),
		IpPermissions: []*ec2.IpPermission{{
			FromPort:   aws.Int64(443),
			ToPort:     aws.Int64(443),
			IpProtocol: aws.String("tcp"),
			IpRanges: []*ec2.IpRange{{
				CidrIp:      aws.String(mmyip),
				Description: aws.String(mdesc),
			}},
		}},
	})
	if err != nil {
		return err
	}
	return nil
}

// Handle a change here
func handleSecurityGroup(entry ToChange, sess *session.Session, ip string) error {

	svc := ec2.New(sess)

	if entry.Role != "" {
		// process
		creds := stscreds.NewCredentials(sess, entry.Role)

		// Create service client value configured for credentials
		// from assumed role.
		svc = ec2.New(sess, &aws.Config{Credentials: creds})
	}

	fmt.Printf("  SecurityGroupID: %s\n", entry.SG)
	fmt.Printf("  IP:              %s\n", ip)
	fmt.Printf("  Port:            %d\n", entry.Port)
	fmt.Printf("  Description:     %s\n", entry.Name)

	deleted, err := myIPDeleteCurrent(svc, entry.SG, ip, entry.Name)
	if err != nil {
		return err
	}
	if deleted {
		fmt.Printf("  Found existing entry, and deleted it\n")
	}

	err = myIPAdd(svc, entry.SG, ip, entry.Name)
	if err != nil {
		return err
	}
	fmt.Printf("  Added entry with current details\n")
	return nil
}

// Execute is invoked if the user chooses this sub-command.
func (i *remoteIPChangeCommand) Execute(args []string) int {

	// Ensure we have a configuration file
	if len(args) < 1 {
		fmt.Printf("Usage: aws-utils remote-ip-change config.json\n")
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

	// Build up a list of rules
	if err = json.Unmarshal(cnf, &changes); err != nil {
		fmt.Printf("Error loading JSON: %s\n", err)
		return 1
	}

	// Get our remote IP
	ip, err := getIP()
	if err != nil {
		fmt.Printf("Error finding your public IP: %s\n", err)
		return 1
	}
	fmt.Printf("Your remote IP is %s\n", ip)

	// Connect to AWS
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-central-1")},
	)
	if err != nil {
		fmt.Printf("Error creating AWS client: %s\n", err)
		return 1
	}

	// Create a new AWS session
	sess, err2 := session.NewSession(&aws.Config{})
	if err2 != nil {
		fmt.Printf("AWS login failed: %s\n", err2.Error())
		return 1
	}

	// Process each group
	for _, entry := range changes {
		err := handleSecurityGroup(entry, sess, ip)
		if err != nil {
			fmt.Printf("error updating %s\n", err)
		}
	}
	return 0
}
