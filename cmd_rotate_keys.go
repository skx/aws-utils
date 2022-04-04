// Rotate the AWS access-keys, and update ~/.aws/credentials with the
// new details.
//
// Only complication here is that we're limited by the number of keys
// we have.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/skx/aws-utils/utils"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

// Structure for our options and state.
type rotateKeysCommand struct {

	// Should we force deletion of old keys?
	Force bool

	// Configuration path, defaults to ~/.aws/credentials
	Path string
}

// Arguments adds per-command args to the object.
func (r *rotateKeysCommand) Arguments(f *flag.FlagSet) {

	f.BoolVar(&r.Force, "force", false, "Should we force removal of old keys without prompting?")
	f.StringVar(&r.Path, "path", "", "The location of the configuration file to modify?")

}

// Info returns the name of this subcommand.
func (r *rotateKeysCommand) Info() (string, string) {
	return "rotate-keys", `Rotate your AWS access keys.

Details:

This command will attempt to generate a new set of AWS access keys, updating
~/.aws/credentials with the new details.

AWS will only allow you to generate two sets of access-credentials, so before
we generate a new public/private key we'll retrieved the existing set.  If you
have two sets already an existing entry must be removed.

Rather than blindly removing an existing set of credentials you will be
asked to confirm via an interactive prompt - unless you add '-force' to
your command-line.

NOTE:

This sub-cmmand has only been tested with essentially "empty" credentials
file which looks like so:

[default]
aws_access_key_id=XFDKLFJDSLFDSF
aws_secret_access_key=3w39r8w0e9r8we09r8ewr90we8r09ew

If you've got a more complex setup I'd urge you to take a backup before you
execute this tool for the first time.
`

}

func (r *rotateKeysCommand) confirm() error {

	// Warning
	fmt.Printf("%s", colorRed)
	fmt.Printf("You already have 2 access keys in-use, we cannot generate more.\n")
	fmt.Printf("\n")
	fmt.Printf("Press Ctrl-C to cancel, or enter 'OK' (uppercase) to delete the oldest key.\n")
	fmt.Printf("\n")
	fmt.Printf("%s", colorReset)

	// Read a line of input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read from STDIN %s", err)
	}

	// Strip CR/newline from string, then leading/trailing spaces
	input = strings.TrimSuffix(input, "\n")
	input = strings.TrimSuffix(input, "\r")
	input = strings.TrimSpace(input)

	// no input
	if input == "" {
		return fmt.Errorf("aborting as you hit enter")
	}

	// not "OK"
	if input != "OK" {
		return fmt.Errorf("aborting as you entered '%s', not 'OK'", input)
	}

	// OK the user confirmed
	return nil
}

// setupPath populates the default path to the configuration file.
//
//
func (r *rotateKeysCommand) setupPath() error {

	// If the user specified a path, use it.
	if r.Path != "" {
		return nil
	}

	// Is there an environmental variable?
	path := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if path != "" {
		r.Path = path
		return nil
	}

	// Otherwise infer via the current user
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to find current user: %s", err)
	}

	// Get the home-directory
	dir := usr.HomeDir

	// Join the path
	r.Path = filepath.Join(dir, ".aws", "credentials")
	return nil
}

// Execute is invoked if the user specifies this sub-command.
func (r *rotateKeysCommand) Execute(args []string) int {

	// Discover a sane path to the credentials file
	err := r.setupPath()
	if err != nil {
		fmt.Printf("error finding credential path %s\n", err)
		return 1
	}

	// Start an AWS session
	var sess *session.Session
	sess, err = utils.NewSession()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return 1
	}

	// Create the handle and list keys - so we can see if we need to
	// remove an existing key before generating a fresh one.
	iamClient := iam.New(sess)
	keys, err := iamClient.ListAccessKeys(&iam.ListAccessKeysInput{})
	if err != nil {
		fmt.Printf("error listing current keys: %s", err)
		return 1
	}

	// More than one key?
	if len(keys.AccessKeyMetadata) > 1 {

		// If we're not forcing ..
		if !r.Force {

			// Then ensure the user confirms removal.
			err = r.confirm()
			if err != nil {
				fmt.Printf("%s\n", err)
				return 1
			}
		}

		// Now remove the older key.
		_, err = iamClient.DeleteAccessKey(&iam.DeleteAccessKeyInput{
			AccessKeyId: keys.AccessKeyMetadata[0].AccessKeyId,
		})

		// Abort on error
		if err != nil {
			fmt.Printf("error deleting oldest key: %s\n", err)
			return 1
		}

	}

	// Open the existing credentials file
	file, err := os.Open(r.Path)
	if err != nil {
		fmt.Printf("couldn't open file %s for reading: %s\n", r.Path, err)
		return 1
	}
	defer file.Close()

	// Create a new key now.
	created, err := iamClient.CreateAccessKey(&iam.CreateAccessKeyInput{})
	if err != nil {
		fmt.Printf("error creating new keys: %s\n", err)
		return 1
	}

	// Collect the lines from within the existing credentials file
	content := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content = append(content, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("error processing the config file: %s\n", err)
		return 1
	}

	// Now create a new file and output our updated values there
	out, err2 := os.Create(r.Path)
	if err2 != nil {
		fmt.Printf("created new keys, but couldn't open output file %s: %s\n", r.Path, err2)
		return 1
	}

	awsAccessKeyID := false
	awsSecretAccessKeyID := false

	// Rewrite here.
	for _, line := range content {

		// Update in-place
		if !awsAccessKeyID && strings.HasPrefix(line, "aws_access_key_id") {

			// only update the first one
			_, err := out.WriteString("aws_access_key_id=" + *created.AccessKey.AccessKeyId + "\n")
			if err != nil {
				fmt.Printf("error writing to file:%s\n", err.Error())
				return 1
			}
			awsAccessKeyID = true
			continue
		}

		// Update in-place
		if !awsSecretAccessKeyID && strings.HasPrefix(line, "aws_secret_access_key") {
			_, err := out.WriteString("aws_secret_access_key=" + *created.AccessKey.SecretAccessKey + "\n")
			if err != nil {
				fmt.Printf("error writing to file:%s\n", err.Error())
				return 1
			}

			awsSecretAccessKeyID = true
			continue
		}

		// Otherwise copy the old line into place.
		_, err := out.WriteString(line + "\n")
		if err != nil {
			fmt.Printf("error writing to file:%s\n", err.Error())
			return 1
		}

	}

	// Close the output file
	out.Close()

	return 0
}
