[![Go Report Card](https://goreportcard.com/badge/github.com/skx/aws-utils)](https://goreportcard.com/report/github.com/skx/aws-utils)
[![license](https://img.shields.io/github/license/skx/aws-utils.svg)](https://github.com/skx/aws-utils/blob/master/LICENSE)
[![Release](https://img.shields.io/github/release/skx/aws-utils.svg)](https://github.com/skx/aws-utils/releases/latest)

# AWS Utils

This repository contains a simple CLI utility with a number of useful sub-commands for working with AWS, particularly for scripting and automation purposes.


## Motivation

Several of the things that this tool does are possible via existing AWS utilities, such as the `aws-cli` package.  However the difference in this tool is that it allows working across roles as a single command - so rather than running an export/list command 20+ times in the traditional way, you can run one command passing the list of roles to use, and all output will be created at once.

Other sub-commands are just more useful, for example listing the available cloudformation stack-names with `aws-cli` will include deleted ones too, which need to be filtered out.  Or allowing all stacks to be updated with a stack protection policy as a single command is just a time-saver.



## Installation

If you the golang development tools installed upon your host, and you're running a recent version, you should be able to download and install via:

```
go install github.com/skx/aws-utils@latest
```

Or, after having cloned [this repository](https://github.com/skx/aws-utils) to your system, you can build from source with a simple:

```
go build .
go install .
```

If you don't wish to build from source you should be able to find precompiled binaries for several operating systems upon our [releases page](https://github.com/skx/aws-utils/releases/)

The binary contains embedded support for bash-completion, to enable this add the following to your bash startup-file:

```
source <(aws-utils bash-completion)
```



## Help

There is integrated help for each sub-command, for example running with no arguments will show you available commands:

```sh
$ aws-utils
Please specify a valid subcommand, choices are:

	bash-completion Generate and output a bash completion-script.
	commands        Show all available sub-commands.
	csv-instances   Export a summary of running instances.
	help            Show usage information.
	instances       Export a summary of running instances.
	orphaned-zones  Show orphaned Route53 zones.
	rotate-keys     Rotate your AWS access keys.
	sg-grep         Security-Group Grep
	stacks          List all cloudformation stack-names.
	subnets         List subnets in all VPCs.
	version         Show the version of this binary.
	whitelist-self  Update security-groups with your external IP.
	whoami          Show the current AWS user or role name.
```

Reading the help text, recommended, is down via the `help` sub-command:

```
$ aws-utils help whitelist-self

Synopsis:
	Update security-groups with your external IP.

Details:

Assume you have some security-groups which contain allow-lists of single IPs.
This command allows you to quickly and easily update those to keep your own
entry current.

...
```


## Common Features

All of the commands accept the use of AWS credentials in the way you'd expect, be it from `~/.aws/credentials` or via the use of environmental-variables:

* For authentication
   * `AWS_ACCESS_KEY_ID`
   * `AWS_SECRET_ACCESS_KEY`
   * `AWS_SESSION_TOKEN` (optionally)
* `AWS_SHARED_CREDENTIALS_FILE`
  * The path to a credentials file (`~/.aws/credentials` by default).
  * Only used by the [rotate-keys](#rotate-keys) sub-command.
* `AWS_REGION`
  * The region to use.

These values are documented in the Golang SDK page:

* https://docs.aws.amazon.com/sdk-for-go/api/aws/session/

Many of the utilities also allow you to operate upon an arbitrary number of AWS roles.  In that case you'd specify the path to a file containing roles to assume, via the `-roles` argument.

For example:

```
$ aws-utils csv-instances -roles=/path/to/roles
```

The format of the file is one-role per line, such as:

```
arn:aws:iam::123457000001:role/foo-AdministratorAccessFromInt-1ABCDEFGHIJKL
arn:aws:iam::123457000002:role/foo-AdministratorAccessFromInt-2ABCDEFGHIJKL
arn:aws:iam::123457000003:role/tst-AdministratorAccessFromInt-3ABCDEFGHIJKL
arn:aws:iam::123457000004:role/tst-AdministratorAccessFromInt-4ABCDEFGHIJKL

# Lines prefixed with "#" are comments, and are ignored (as are empty-lines).
```



## SubCommands

The following sub-commands are available:

* [csv-instances](#csv-instances)
* [instances](#instances)
* [orphaned-zones](#orphaned-zones)
* [rotate-keys](#rotate-keys)
* [sg-grep](#sg-grep)
* [stacks](#stacks)
* [subnets](#subnets)
* [whitelist-self](#whitelist-self)
* [whoami](#whoami)





### `csv-instances`

Output a list of running instances, as CSV.  The output may be changed, but by default we show:

* Account ID
* Instance ID
* Instance Name
* AMI ID

Usage:

```sh
$ aws-utils csv-instances [-roles=/path/to/roles]
```

The fields displayed may be changed via the `format` argument, for example:

```sh
$ aws-utils csv-instances --format="name,id,subnet,subnetid,vpc,vpcid"
```

The list of available field-names can be viewed via `aws-utils help csv-instances`.


### `instances`

Show a human-readable list of all the EC2 instances you have running, along
with details of the volumes associated with each instance.

Sample output:

```
i-01066633e12345567 - prod-fooapp-uk
------------------------------------
	AMI          : ami-01234567890abcdef
    AMI Age      : 4 days
	Instance type: t3.medium
	Key name     : sysadmin1
	Private IPv4 : 10.30.44.105
	Volumes:
		/dev/sda1	vol-01234567890abcdef	100Gb	gp2	Encrypted:true	IOPs:300
```

Usage:

```sh
$ aws-utils instances [-json] [-roles=/path/to/roles]
```

The output is defined by a simple golang template.  If you wish to change the template you can do so:

```sh
$ aws-utils instances -dump-template > foo.tmpl
$ vi foo.tmpl
$ aws-utils instances -template=./foo.tmpl
```


### `orphaned-zones`

This sub-command examines all domains which have DNS hosted in Route53,
and reports those which have nameservers configured which do __not__
belong to AWS.

This is designed to identify domains which have expired, or had their
DNS-hosting moved to an external system (such as cloudflare, or similar).

Usage:

```sh
$ aws-utils orphaned-zones
VALID  - dhcp.io.
ORPHAN - example.com.
```



### `rotate-keys`

This sub-command uses the AWS API to regenerate a new set of AWS access-keys,
and updates your `~/.aws/credentials` file with the new values.

**NOTE**:

* You may only have two sets of AWS Access Keys at a time
  * So if you have two already one must be removed.
  * You will be prompted prior to the removal of one, or you can add `-force` to avoid that interactive prompt.
* `~/.aws/credentials` is the default file to use as the template for updating
  * If that file is missing your keys will be removed/created but they will then be lost.
  * This is because the output is achieved by reading the existing file and replacing existing keys, rather than blindly overwriting.
  * We want to do this to avoid data-loss on things like your profile(s) and other configuration values.
* **Take a backup** before running this tool for the first time.



### `sg-grep`

Show security-groups which match a particular regular expression.

```
$ aws-utils sg-grep 0.0.0.0/0
sg-01234567890abcdef [eu-central-1] - launch-wizard-1 created 2021-11-19T09:39:15.473+02:00
	{
	  Description: "launch-wizard-1 created 2021-11-19T09:39:15.473+02:00",
	  GroupId: "sg-sg-01234567890abcdef",
	  GroupName: "launch-wizard-1",
	  IpPermissions: [{
	      FromPort: 22,
	      IpProtocol: "tcp",
	      IpRanges: [{
	          CidrIp: "0.0.0.0/0",
	          Description: ""
	        }],
	      ToPort: 22
	    }],

```

Usage:

```sh
$ aws-utils sg-grep [-roles=/path/to/roles] search-term1 search-term2 ..
```



### `stacks`

Show the names, and optionally the statuses of all cloudformation stacks.

This is useful for applying stack-policies to a list of stacks, for example, and avoids the use of more complex CLI invocations when using the AWS CLI.

You can also update all stacks with a protection policy in a single operation.

Usage:

```sh
$ aws-utils stacks
StackSet-09c62176-4401-4c2e-b018-b3983c37619d
my-prod--iam
my-prod--lifecycel-manager
my-prod--route53
..

$ aws-utils stacks -policy ./my-stack-policy.json
```

Optionally you may display the stack-status, and include deleted-stacks.




### `subnets`

Show the subnets, along with associated CIDR ranges, available within all VPCs.

This is useful when you're running a penetration test, or want a quick overview of all the available subnets across a bunch of accounts.

Usage:

```sh
$ aws-utils subnets
Account, VPC, Subnet Name, Subnet ID, Cidr
207250808959,vpc-bbe705d2,default-eu-central-1a,subnet-b4df30dd,172.31.16.0/20
207250808959,vpc-bbe705d2,default-eu-central-1b,subnet-406c6238,172.31.0.0/20
207250808959,vpc-bbe705d2,default-eu-central-1a,subnet-44fad80e,172.31.32.0/20
```




### `whitelist-self`

This sub-command allows you to quickly update Ingress rules, with your current external IP address.

Imagine you have a number of security-groups which permit access to resources via a small list of permitted source IPs this command will let you update your own entry in that list easily.

Sample input file:

```
$ cat input.json
[
  { "SG": "sg-12344", "Name": "[aws-utils] Steve's Home IP", "Port": 443 },
  { "SG": "sg-12345", "Name": "[aws-utils] Steve's Home IP", "Port": 22 }
]
```

Valid values for the JSON object are:

* `Display`
  * A message to display when the group is updated, good for documentation.
* `SG`
  * The ID of the security-group to update.
* `Name`
  * The name of the rule to add (i.e. description)
  * This **must** be unique within the security-group.
* `Port`
  * The port to permit.
* `Role`
  * Optionally you may specify an ARN of a role to assume.
  * example : `arn:aws:iam::123456789010:role/devops-access`


As you can see each rule allows you to whitelist a single port, and only a single port.  Of course if you wish you can repeat rules with different ports like so:

```json
[
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] ${USER} home: HTTPS",
        "Port": 443
    },
    {
        "SG": "sg-12345",
        "Name": "[aws-utils] ${USER} home: SSH",
        "Port": 22
    }
]
```

Once you run the tool, with a suitable JSON input file, you'll get output like so:

```
$ ./aws-utils whitelist-self ./prod.json
Your remote IP is 191.145.83.183/32
  SecurityGroupID: sg-12345
  IP:              191.145.83.183/32
  Port:            443
  Description:     [aws-utils] steve home: HTTPS
  Found existing entry, and deleted it.
  Added entry with current details.
```

Usage:

```sh
$ aws-utils whitelist-self /path/to/rules.json
```


### `whoami`

Show the current user, or assumed role.

```
$ aws-utils whoami
aws-company-devops-prd
```

Or having assumed a role:

```
$ aws-utils whoami
aws-company-role-prod-ro
```



## Github Setup

This repository is configured to run tests upon every commit, and when
pull-requests are created/updated.  The testing is carried out via
[.github/run-tests.sh](.github/run-tests.sh) which is used by the
[github-action-tester](https://github.com/skx/github-action-tester) action.

Releases are automated in a similar fashion via [.github/build](.github/build),
and the [github-action-publish-binaries](https://github.com/skx/github-action-publish-binaries) action.
