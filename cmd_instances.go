// Show details of running instances, along with their volumes.
//
// Primarily written to answer support-questions.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/template"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/skx/aws-utils/instances"
	"github.com/skx/aws-utils/utils"
)

// Structure for our options and state.
type instancesCommand struct {

	// Path to a file containing roles
	rolesPath string

	// Should we export our results in JSON format?
	jsonOutput bool

	// Should we dump the default template?
	dumpTemplate bool

	// Specify a non-default template?
	templatePath string
}

// Arguments adds per-command args to the object.
func (i *instancesCommand) Arguments(f *flag.FlagSet) {
	f.StringVar(&i.rolesPath, "roles", "", "Path to a list of roles to process, one by one.")
	f.StringVar(&i.templatePath, "template", "", "Path to a template to render, instead of the default")
	f.BoolVar(&i.dumpTemplate, "dump-template", false, "Output the standard template to the console, and terminate")
	f.BoolVar(&i.jsonOutput, "json", false, "Output the results in JSON.")
}

// Info returns the name of this subcommand.
func (i *instancesCommand) Info() (string, string) {
	return "instances", `Export a summary of running instances.

Details:

This command exports details about running instances, in a human-readable
fashion.  For example this is how a single instance is described by default:

aviatrix-gateway - i-012345679abcdef01
-----------------------------------------------------------
        AMI          : ami-0d3ba21723ec0dc5d
        AMI Age      : 3 days
        Instance type: t2.small
        Public  IPv4 address: 3.127.201.130
        Private IPv4 address: 10.10.3.78
        Volumes:
          /dev/sda1  vol-05c23836682aceab8      Size:16GiB      IOPS:100

The output of this command is generated by a standard text-template, and
if you wish to customize the output you can specify the path to a template
to use.

The default template can be displayed by running:

    $ aws-utils instances -dump-template

With the default template you can make your changes and use it like so:

    $ aws-utils instances -dump-template > foo.tmpl
    $ vi foo.tmpl
    $ aws-utils instances -template=./foo.tmpl
    ..
`

}

// DumpInstances looks up the appropriate details and outputs them to the
// console, via the use of a provided template.
func (i *instancesCommand) DumpInstances(svc *ec2.EC2, acct string, void interface{}) error {

	// Cast our template back into the correct object-type
	tmpl := void.(*template.Template)

	// Get the instances that are running.
	ret, err := instances.GetInstances(svc, acct)
	if err != nil {
		return err
	}

	// For each one, output the appropriate thing.
	for _, obj := range ret {

		// Output the rendered template to the console
		if i.jsonOutput {

			var b []byte
			b, err = json.Marshal(obj)
			if err != nil {
				return fmt.Errorf("error exporting to JSON %s", err)
			}
			fmt.Println(string(b))
		} else {
			err = tmpl.Execute(os.Stdout, ret)
			if err != nil {
				return fmt.Errorf("error rendering template %s", err)
			}
		}
	}
	return nil
}

// Execute is invoked if the user specifies this subcommand.
func (i *instancesCommand) Execute(args []string) int {

	//
	// Create the template we'll use for output
	//
	text := `
{{.InstanceName}} {{.InstanceID}}
  AMI         : {{.InstanceAMI}}
  AMI Age     : {{.AMIAge}} days
  AWS Account : {{.AWSAccount}}
{{- if .SSHKeyName  }}
  KeyName     : {{.SSHKeyName}}
{{- end}}
{{- if .PrivateIPv4 }}
  Private IPv4: {{.PrivateIPv4}}
{{- end}}
{{- if .PublicIPv4  }}
  Public  IPv4: {{.PublicIPv4}}
{{- end}}
{{if .Volumes}}
  Volumes:{{range .Volumes}}
     {{.Device}} {{.ID}} Size:{{.Size}}GiB Type:{{.Type}} Encrypted:{{.Encrypted}} IOPS:{{.IOPS}}{{end}}
{{end}}
`

	// Show the template?
	if i.dumpTemplate {
		fmt.Printf("%s\n", text)
		return 0
	}

	// If the user specified a template-path then use it
	if i.templatePath != "" {
		content, err := os.ReadFile(i.templatePath)
		if err != nil {
			fmt.Printf("failed to read %s:%s\n", i.templatePath, err.Error())
			return 1

		}

		text = string(content)
	}

	// Compile the template
	tmpl, err := template.New("output").Parse(text)
	if err != nil {
		fmt.Printf("failed to compile template:%s\n", err.Error())
		return 1
	}

	//
	// Get the connection, using default credentials
	//
	session, err := utils.NewSession()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return 1
	}

	//
	// Now invoke our callback - this will call the function
	// "DumpInstances" once if we're not running with a role-file,
	// otherwise once for each role.
	//
	errs := utils.HandleRoles(session, i.rolesPath, i.DumpInstances, tmpl)

	if len(errs) > 0 {
		fmt.Printf("errors running instance dump\n")
		for _, err := range errs {
			fmt.Printf("%s\n", err)
		}
		return 1
	}

	return 0
}
