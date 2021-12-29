// Show details of running instances, along with their volumes.
//
// Primarily written to answer support-questions.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type instancesCommand struct {

	// Path to a file containing roles
	rolesPath string
}

// Volume holds detailed regarding an instances volumes
type Volume struct {
	// Device is the name of the device
	Device string

	// ID is the name of the ID
	ID string

	// Size is the size of the device.
	Size string

	// Type is the storage type.
	Type string

	// Encrypted contains the encryption value of the volume
	Encrypted string

	// IOPS holds the speed of the device.
	IOPS string
}

// InstanceOutput is the structure used to populate our templated output
type InstanceOutput struct {
	// InstanceID holds the AWS instance ID
	InstanceID string

	// InstanceName holds the AWS instance name, if set
	InstanceName string

	// InstanceAMI holds the AMI name
	InstanceAMI string

	// InstanceState holds the instance state (stopped, running, etc)
	InstanceState string

	// InstanceType holds the instance type "t2.tiny", etc.
	InstanceType string

	// Keypair setup for access.
	SSHKeyName string

	// PublicIPv4 has the public IPv4 address
	PublicIPv4 string

	// PrivateIPv4 has the private IPv4 address
	PrivateIPv4 string

	// Volumes holds all known volumes
	Volumes []Volume
}

// Arguments adds per-command args to the object.
func (c *instancesCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&c.rolesPath, "roles", "", "Path to a list of roles to process, one by one")
}

// Info returns the name of this subcommand.
func (c *instancesCommand) Info() (string, string) {
	return "instances", `Export a summary of running instances.

Details:

This command exports details about running instances, in a human-readable
fashion.

aviatrix-sre-prd-rss-aviatrix-gateway - i-047673c09867d3c3a
-----------------------------------------------------------
        AMI: ami-0d3ba21723ec0dc5d
        Instance type: t2.small
        Public  IPv4 address: 3.127.201.130
        Private IPv4 address: 10.10.3.78
        State: running
        Volumes:
        vol-05c23836682aceab8 attached as /dev/sda1     Size:16GiB      IOPS:100
`

}

// Dump looks up the appropriate details and outputs them to the console, via the use
// of the provided template.
func Dump(svc *ec2.EC2, acct string, tmpl *template.Template) error {

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

		// The structure to output for this instance
		var out InstanceOutput

		for _, instance := range reservation.Instances {

			// We have a running EC2 instance, we'll populate
			// the InstanceOutput structure with details which we
			// can then print using a simple template.
			//

			// Values which are always present.
			out.InstanceID = *instance.InstanceId
			out.InstanceName = *instance.InstanceId
			out.InstanceState = *instance.State.Name
			out.InstanceType = *instance.InstanceType
			out.InstanceAMI = *instance.ImageId

			// Look for the name, which is set via a Tag.
			i := 0
			for i < len(instance.Tags) {

				if *instance.Tags[i].Key == "Name" {
					out.InstanceName = *instance.Tags[i].Value
				}
				i++
			}

			// Optional values
			if instance.KeyName != nil {
				out.SSHKeyName = *instance.KeyName
			}
			if instance.PublicIpAddress != nil {
				out.PublicIPv4 = *instance.PublicIpAddress
			}
			if instance.PrivateIpAddress != nil {
				out.PrivateIPv4 = *instance.PrivateIpAddress
			}

			// Now the storage associated with the instance
			vols, err := readBlockDevicesFromInstance(instance, svc)
			if err == nil {
				for _, x := range vols["ebs"].([]map[string]interface{}) {

					out.Volumes = append(out.Volumes, Volume{
						Device:    fmt.Sprintf("%s", x["device_name"]),
						ID:        fmt.Sprintf("%s", x["id"]),
						Size:      fmt.Sprintf("%s", x["volume_size"]),
						Type:      fmt.Sprintf("%s", x["volume_type"]),
						Encrypted: fmt.Sprintf("%s", x["encrypted"]),
						IOPS:      fmt.Sprintf("%s", x["iops"])})
				}
			} else {
				return (fmt.Errorf("failed to read devices %s", err))
			}

			// Output the rendered template to the console
			err = tmpl.Execute(os.Stdout, out)
			if err != nil {
				return fmt.Errorf("error rendering template %s", err)
			}
		}
	}
	return nil
}

func readBlockDevicesFromInstance(instance *ec2.Instance, conn *ec2.EC2) (map[string]interface{}, error) {
	blockDevices := make(map[string]interface{})
	blockDevices["ebs"] = make([]map[string]interface{}, 0)

	instanceBlockDevices := make(map[string]*ec2.InstanceBlockDeviceMapping)
	for _, bd := range instance.BlockDeviceMappings {
		if bd.Ebs != nil {
			instanceBlockDevices[*bd.Ebs.VolumeId] = bd
		}
	}

	if len(instanceBlockDevices) == 0 {
		return nil, nil
	}

	volIDs := make([]*string, 0, len(instanceBlockDevices))
	for volID := range instanceBlockDevices {
		volIDs = append(volIDs, aws.String(volID))
	}

	// Need to call DescribeVolumes to get volume_size and volume_type for each
	// EBS block device
	volResp, err := conn.DescribeVolumes(&ec2.DescribeVolumesInput{
		VolumeIds: volIDs,
	})
	if err != nil {
		return nil, err
	}

	for _, vol := range volResp.Volumes {
		instanceBd := instanceBlockDevices[*vol.VolumeId]
		bd := make(map[string]interface{})

		bd["id"] = *vol.VolumeId
		if instanceBd.Ebs != nil && instanceBd.Ebs.DeleteOnTermination != nil {
			bd["delete_on_termination"] = *instanceBd.Ebs.DeleteOnTermination
		}
		if vol.Size != nil {
			bd["volume_size"] = *vol.Size
		}
		if vol.VolumeType != nil {
			bd["volume_type"] = *vol.VolumeType
		}
		if vol.Iops != nil {
			bd["iops"] = *vol.Iops
		}

		if instanceBd.DeviceName != nil {
			bd["device_name"] = *instanceBd.DeviceName
		}
		if vol.Encrypted != nil {
			bd["encrypted"] = *vol.Encrypted
		}
		if vol.SnapshotId != nil {
			bd["snapshot_id"] = *vol.SnapshotId
		}

		blockDevices["ebs"] = append(blockDevices["ebs"].([]map[string]interface{}), bd)
	}

	return blockDevices, nil
}

// Execute is invoked if the user specifies this subcommand.
func (c *instancesCommand) Execute(args []string) int {

	//
	// Create the template we'll use for output
	//
	text := `
{{.InstanceName}} {{.InstanceId}}

  AMI         : {{.InstanceAMI}}
{{if .SSHKeyName }}  KeyName     : {{.SSHKeyName}}{{end}}
{{if .PrivateIPv4}}  Private IPv4: {{.PrivateIPv4}}{{end}}
{{if .PublicIPv4 }}  Public  IPv4: {{.PublicIPv4}}{{end}}
  State       : {{.InstanceState}}

{{if .Volumes}}
  Volumes:
{{range .Volumes}}
     {{.Id}} mounted on {{.Device}}\tSize:{{.Size}}GiB\tType:{{.Type}}\tEncrypted:{{.Encrypted}}\tIOPS:{{.IOPS}}
{{end}}
{{end}}
`
	tmpl := template.Must(template.New("output").Parse(text))

	//
	// Get the connection, using default credentials
	//
	sess, err2 := utils.NewSession()
	if err2 != nil {
		fmt.Printf("%s\n", err2.Error())
	}

	//
	// Create a new session to find our account
	//
	stsSvc := sts.New(sess)
	out, err3 := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err3 != nil {
		fmt.Printf("Failed to get identity: %s", err3.Error())
		return 1
	}

	acct := *out.Account

	//
	// If we have no role-list then just dump our current account
	//
	if c.rolesPath == "" {

		svc := ec2.New(sess)

		err := Dump(svc, acct, tmpl)
		if err != nil {
			fmt.Printf("error syncing account %s\n", err.Error())
			return 1
		}

		return 0
	}

	//
	// OK we have a list of roles, handle them one by one
	//
	file, err := os.Open(c.rolesPath)
	if err != nil {
		fmt.Printf("Error opening role-file: %s %s\n", c.rolesPath, err.Error())
		return 1
	}
	defer file.Close()

	//
	// Process the role-file line by line
	//
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		// Get the line
		role := scanner.Text()

		// Skip comments
		if strings.HasPrefix(role, "#") {
			continue
		}

		// process
		creds := stscreds.NewCredentials(sess, role)

		// Create service client value configured for credentials
		// from assumed role.
		svc := ec2.New(sess, &aws.Config{Credentials: creds})

		// We'll get the account from the string which looks like this:
		//
		// arn:aws:iam::1234:role/blah-abc
		//
		// We split by ":" and get the fourth field.
		//
		data := strings.Split(role, ":")
		acct := data[4]

		// Process the running instances
		err = Dump(svc, acct, tmpl)
		if err != nil {
			fmt.Printf("Error for role %s %s\n", role, err.Error())
		}
	}

	//
	// Error processing the end of the file?
	//
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error processing role-file: %s %s\n", c.rolesPath, err.Error())
		return 1
	}

	return 0
}
