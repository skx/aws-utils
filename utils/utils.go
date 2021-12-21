// Package utils just contains a couple of common-methods.
//
// This package should not exist.
package utils

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

// NewSession returns an AWS session object, with optional request-tracing.
//
// If the environmental variable "DEBUG" is non-empty then requests made to
// AWS will be logged to the console.
func NewSession() (*session.Session, error) {

	sess, err := session.NewSession()
	if err != nil {
		return sess, err
	}

	debug := os.Getenv("DEBUG")
	if debug != "" {

		// Add a logging handler
		sess.Handlers.Send.PushFront(func(r *request.Request) {
			fmt.Printf("Request: %v [%s] %v",
				r.Operation, r.ClientInfo.ServiceName, r.Params)
		})
	}

	return sess, nil
}
