package main

import (
	"fmt"
	"os"

	"github.com/skx/subcommands"
)

//
// Recovery is good
//
func recoverPanic() {
	if os.Getenv("DEBUG") != "" {
		return
	}

	if r := recover(); r != nil {
		fmt.Printf("recovered from panic while running %v\n%s\n", os.Args, r)
		fmt.Printf("To see the panic run 'export DEBUG=on' and repeat.\n")
	}
}

//
// Register the subcommands, and run the one the user chose.
//
func main() {

	//
	// Catch errors
	//
	defer recoverPanic()

	//
	// Register each of our subcommands.
	//
	subcommands.Register(&csvInstancesCommand{})
	subcommands.Register(&instancesCommand{})
	subcommands.Register(&orphanedZonesCommand{})
	subcommands.Register(&rotateKeysCommand{})
	subcommands.Register(&sgGrepCommand{})
	subcommands.Register(&stacksCommand{})
	subcommands.Register(&whitelistSelfCommand{})
	subcommands.Register(&versionCommand{})
	subcommands.Register(&whoamiCommand{})

	//
	// Execute the one the user chose.
	//
	os.Exit(subcommands.Execute())
}
