// Show the current login details.
//
// Primarily written as a sanity-check when logging in via the sts:assumerole

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/skx/aws-utils/utils"
	"github.com/skx/subcommands"

	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

// Structure for our options and state.
type whoamiCommand struct {

	// We embed the NoFlags option, because we accept no command-line flags.
	subcommands.NoFlags
}

// Info returns the name of this subcommand.
func (i *whoamiCommand) Info() (string, string) {
	return "whoami", `Show the current AWS user or role name.

Details:

This command shows you an overview of who you are current logged into
AWS with, be it a root-user, or an assumed-role.
`

}

func showError() {
	txt := `
No credentials were found - Please see the following AWS link:

   https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-quick-configuration`
	fmt.Printf("%s\n", txt)
	os.Exit(1)
}

func getAccountID(svc stsiface.STSAPI) (id string) {
	callerID, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})

	switch {
	case err != nil:
		if awsErr, okBPA2 := errors.Cause(err).(awserr.Error); okBPA2 {
			if strings.Contains(awsErr.Message(), "non-User credentials") {
				// not using user creds, so need to try a different method
			} else if awsErr.Code() == "NoCredentialProviders" {
				showError()
			} else if awsErr.Code() == "ExpiredToken" {
				fmt.Printf("Your temporary credentials have expired\n")
				os.Exit(1)
			} else if strings.Contains(awsErr.Message(), "security token included in the request is invalid") {
				fmt.Printf("The specified credentials have an invalid security token")
				os.Exit(1)
			} else {
				fmt.Printf("Unknown error using specified credentials: %s\n", awsErr.Message())
			}
		}
	case callerID.Arn == nil:
		showError()
	default:
		id = *callerID.Account
		return
	}
	return id
}

func getAccountAlias(svc iamiface.IAMAPI) (alias string) {

	getAliasOutput, err := svc.ListAccountAliases(&iam.ListAccountAliasesInput{})
	if err != nil {
		fmt.Println("Missing \"iam:ListAccountAliases\" permission so unable to retrieve alias")
	} else if len(getAliasOutput.AccountAliases) > 0 {
		alias = *getAliasOutput.AccountAliases[0]
	}
	return
}

// Execute is invoked if the user specifies this sub-command.
func (i *whoamiCommand) Execute(args []string) int {

	// Start a session
	sess, err := utils.NewSession()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return 1
	}

	// Get the IAM handle, and STS service
	svc := iam.New(sess)
	stsSvc := sts.New(sess)

	// Find the account and alias (optional)
	accountID := getAccountID(stsSvc)
	accountAlias := getAccountAlias(svc)

	// Prefer the alias to the account
	if accountAlias != "" {
		fmt.Printf("%s\n", accountAlias)
	} else {
		fmt.Printf("%s\n", accountID)
	}
	return 0
}
