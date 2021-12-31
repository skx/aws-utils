// Package amiage contains a helper function to return the age of an
// AMI in days.
//
// Caching is supported.
package amiage

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Cache of creation-time/date
var cache map[string]string

// init ensures that our cache is initialized
func init() {
	cache = make(map[string]string)
}

// AMICreation returns the creation-date of the given AMI as a string.
//
// Values are cached.
func AMICreation(svc *ec2.EC2, id string) (string, error) {

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
		return "", fmt.Errorf("error calling DescribeImages: %s", err.Error())
	}

	// If we got a result then we can return the creation time
	// (as a string)
	if len(result.Images) > 0 {

		// But save in a cache for the future
		date := *result.Images[0].CreationDate
		cache[id] = date
		return date, nil
	}
	return "", fmt.Errorf("no creation date for AMI %s", id)
}

// AMIAge returns the number of days since the specified image was created,
// using AMICreation as a helper (that will cache the creation time).
func AMIAge(svc *ec2.EC2, id string) (int, error) {

	//
	// Get the AMI creation-date
	//
	create, err := AMICreation(svc, id)
	if err != nil {
		return 0, fmt.Errorf("failed to get creation date of %s: %s", id, err.Error())
	}

	//
	// Parse the date, so we can report how many days
	// ago the AMI was created.
	//
	t, err := time.Parse("2006-01-02T15:04:05.000Z", create)
	if err != nil {
		return 0, fmt.Errorf("failed to parse time string %s: %s", create, err)
	}

	//
	// Count how old the AMI is in days
	//
	date := time.Now()
	diff := date.Sub(t)

	return (int(diff.Hours() / 24)), nil
}
