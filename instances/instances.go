// Package instances contains the common code which will find
// running EC2 instances, and return details about them.
package instances

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/amiage"
)

// Volume holds detailed regarding an instances volumes.
//
// This structure is used to populate the text/template we use for output
// generation.
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
//
// This structure is used to populate the text/template we use for output
// generation.
type InstanceOutput struct {

	// AWSAccount is the account number we're running under
	AWSAccount string

	// InstanceID holds the AWS instance ID
	InstanceID string

	// InstanceName holds the AWS instance name, if set
	InstanceName string

	// InstanceAMI holds the AMI name
	InstanceAMI string

	// AMIAge contains the age of the AMI in days.
	AMIAge int

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

// GetInstances returns details about our running instances.
func GetInstances(svc *ec2.EC2, acct string) ([]InstanceOutput, error) {

	// Our return value
	ret := []InstanceOutput{}

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
		return ret, fmt.Errorf("DescribeInstances failed: %s", err)
	}

	// For each instance build up an object to describe it
	for _, reservation := range result.Reservations {

		// The structure to output for this instance
		var out InstanceOutput

		for _, instance := range reservation.Instances {

			// We have a running EC2 instance, we'll populate
			// the InstanceOutput structure with details.

			// Values which are always present.
			out.AWSAccount = acct
			out.InstanceID = *instance.InstanceId
			out.InstanceName = *instance.InstanceId
			out.InstanceState = *instance.State.Name
			out.InstanceType = *instance.InstanceType
			out.InstanceAMI = *instance.ImageId

			// Get the AMI age, in days.
			out.AMIAge, err = amiage.AMIAge(svc, out.InstanceAMI)
			if err != nil {
				return ret, fmt.Errorf("error getting AMI age for %s: %s", out.InstanceAMI, err)
			}

			// Look for the name, which is set via a Tag.
			n := 0
			for n < len(instance.Tags) {

				if *instance.Tags[n].Key == "Name" {
					out.InstanceName = *instance.Tags[n].Value
				}
				n++
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
						Size:      fmt.Sprintf("%d", x["volume_size"]),
						Type:      fmt.Sprintf("%s", x["volume_type"]),
						Encrypted: fmt.Sprintf("%t", x["encrypted"]),
						IOPS:      fmt.Sprintf("%d", x["iops"])})
				}
			} else {
				return ret, fmt.Errorf("failed to read devices %s", err)
			}

			ret = append(ret, out)
		}
	}

	return ret, nil
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
