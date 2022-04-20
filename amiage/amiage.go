// Package amiage contains a helper function to return the age of an
// AMI in days.
//
// Caching is supported.
package amiage

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Cache of creation-time/date
var cache map[string]string

// NotFound is the error returned when an AMI isn't found
var NotFound error

// init ensures that our cache is initialized
func init() {
	cache = make(map[string]string)
	NotFound = fmt.Errorf("not-found")
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

	// Run the search, return NotFound if we got an error.
	//
	// Yes we're losing the "real" error here, but logically we've not found the
	// details we want, so it's a not-found.
	result, err := svc.DescribeImages(input)
	if err != nil {
		return "", NotFound
	}

	// If we got a result then we can return the creation time (as a string)
	if len(result.Images) > 0 {

		// But save in a cache for the future
		date := *result.Images[0].CreationDate
		cache[id] = date
		return date, nil
	}

	// No result found
	return "", NotFound
}

// AMIAge returns the number of days since the specified image was created,
// using AMICreation as a helper (that will cache the creation time).
func AMIAge(svc *ec2.EC2, id string) (int, error) {

	//
	// Get the AMI creation-date
	//
	create, err := AMICreation(svc, id)
	if err != nil {

		// If this is "Not Found" then return -1
		if errors.Is(err, NotFound) {
			return -1, err
		}
		return -2, fmt.Errorf("failed to get creation date of %s: %s", id, err.Error())
	}

	//
	// Parse the date, so we can report how many days
	// ago the AMI was created.
	//
	t, err := time.Parse("2006-01-02T15:04:05.000Z", create)
	if err != nil {
		return -3, fmt.Errorf("failed to parse time string %s: %s", create, err)
	}

	//
	// Count how old the AMI is in days
	//
	date := time.Now()
	diff := date.Sub(t)

	return (int(diff.Hours() / 24)), nil
}
