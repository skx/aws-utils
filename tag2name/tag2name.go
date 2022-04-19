// Package tag2name contains a helper which will retrieve
// the name of a "thing", by examining the values of each
// tag.
//
// An AWS resource may contain numerous tags, but the name
// of the thing will be retrieved from the tag named "Name".
package tag2name

import (
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Lookup will return the name of a "thing" from the given set of tags.
// If there is no name found the fallback value will be returned instead.
func Lookup(tags []*ec2.Tag, fallback string) string {

	n := 0
	for n < len(tags) {

		if *tags[n].Key == "Name" {
			return *tags[n].Value
		}
		n++
	}

	return fallback
}
