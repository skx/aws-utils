// Package utils just contains a couple of helper-methods which simplify
// the implementation of our various sub-commands.
package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
)

// AWSCallback is the signture of a function which can be invoked
// against either the default AWS account, or against a list of
// named roles assumed from it, in order.
type AWSCallback func(svc *ec2.EC2, account string, void interface{}) error

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

// HandleRoles invokes the specified callback, handling the case where
// a role-file is specified or not.
//
// If the roleFile is empty then the function will be invoked once,
// otherwise it will be invoked for every role.
//
// To allow execution to continue on subsequent roles errors in the execution
// of a callback do not cause processing of the callback to terminate.
func HandleRoles(session *session.Session, roleFile string, callback AWSCallback, void interface{}) []error {

	// We collect errors, and continue operating
	var errs []error

	//
	// Create a new session to find our account
	//
	svc := sts.New(session)
	out, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return []error{fmt.Errorf("failed to get identity: %s", err)}
	}

	//
	// This is our (default) account ID
	//
	acct := *out.Account

	//
	// If we have no role-list then just run the callback once,
	// using the default credentials.
	//
	if roleFile == "" {

		// Get the EC2 helper
		svc := ec2.New(session)

		// Run the callback
		err = callback(svc, acct, void)

		// append the error, if we got it.
		if err != nil {
			errs = append(errs, fmt.Errorf("error invoking callback: %s", err))
		}

		// return any errors we received
		return errs
	}

	//
	// OK we have a list of roles, handle them one by one
	//
	file, err := os.Open(roleFile)
	if err != nil {
		return []error{fmt.Errorf("failed to open role-file %s: %s", roleFile, err)}
	}
	defer file.Close()

	//
	// Process the role-file line by line
	//
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		// Get the line, and trim leading/trailing spaces
		role := scanner.Text()
		role = strings.TrimSpace(input)

		// Skip comments
		if strings.HasPrefix(role, "#") {
			continue
		}

		// Skip lines that aren't well-formed
		if !strings.HasPrefix(role, "arn:") && !strings.HasPrefix(role, "ARN:") {
			continue
		}

		// Assume the role
		creds := stscreds.NewCredentials(session, role)

		// Create service client value configured for credentials
		// from assumed role.
		svc := ec2.New(session, &aws.Config{Credentials: creds})

		// We'll get the account from the string which looks like this:
		//
		// arn:aws:iam::1234:role/blah-abc
		//
		// We split by ":" and get the fourth field.
		//
		data := strings.Split(role, ":")
		acct := data[4]

		// invoke the callback for this account.
		err := callback(svc, acct, void)

		// If we got an error keep going, but save it away.
		if err != nil {
			errs = append(errs, fmt.Errorf("error invoking callback: %s", err))
		}
	}

	//
	// Error processing the end of the file?
	//
	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Errorf("error processing role-file: %s %s", roleFile, err))
	}

	// return any errors we've built up
	return errs
}
