# AWS Utils

This repository contains a simple CLI utility with a number of useful sub-commands implemented for working with AWS


## Installation

Download a binary from our release-page, or build from source.

Once installed you'll find the binary `aws-utils`, which you can execute from your shell.

There is support for bash-completion, to enable this add the following to your bash startup-file:

```
        source <(aws-utils bash-completion)
```




## Common Features

All of the commands accept the use of AWS credentials in the way you'd expect, be it from `~/.aws/credentials` or via the use of environmental-variables:

* AWS_SECRET_ACCESS_KEY
* AWS_ACCESS_KEY_ID
* AWS_SESSION_TOKEN
  * For the cases when you're using assume-role.
* AWS_REGION
  * The region to use.

This is documented in the Golang SDK page:

* https://docs.aws.amazon.com/sdk-for-go/api/aws/session/

Many of the utilities also allow you to operate the same functionality upon an arbitrary number of AWS roles.  In that case you'd specify the path to a file containing roles to assume, via the `-roles` argument.

For example:

```
aws csv-instances -roles=/path/to/roles
```

The format of the file is one-role per line, such as:

```
arn:aws:iam::123457000001:role/foo-AdministratorAccessFromInt-1ABCDEFGHIJKL
arn:aws:iam::123457000002:role/foo-AdministratorAccessFromInt-2ABCDEFGHIJKL
arn:aws:iam::123457000003:role/tst-AdministratorAccessFromInt-3ABCDEFGHIJKL
arn:aws:iam::123457000004:role/tst-AdministratorAccessFromInt-4ABCDEFGHIJKL

# Lines prefixed with "#" are comments, and are ignored.

```



## SubCommands

The following sub-commands are available:

* [csv-instances](#csv-instances)
* [instances](#instances)
* [sg-grep](#sg-grep)
* [whoami](#whoami)





### `csv-instances`

Export a simple CSV-based list of running instances:

* Account ID
* Instance ID
* Instance Name
* AMI ID
* Age of AMI in days



### `instances`

Show a human-friendly list of all the EC2 instances you have running.

Sample output:

```
i-01066633e12345567 - prod-fooapp-uk
------------------------------------
	AMI: ami-01234567890abcdef
	Instance type: t3.medium
	Key name: sysadmin1
	Private IPv4: 10.30.44.105
	State: running
	Volumes:
		/dev/sda1	vol-01234567890abcdef	100Gb	gp2	Encrypted:true	IOPs:300
```



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
