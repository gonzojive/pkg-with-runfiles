package main

import (
	"fmt"
	"os"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func main() {
	rfiles, err := runfiles.New()
	if err != nil {
		fmt.Printf("error loading runfiles: %v\n", err)
		os.Exit(1)
	}
	loc, err := rfiles.Rlocation("data1_from_rooty/message.txt")
	if err != nil {
		fmt.Printf("error determining runfile message.txt: %v\n", err)
		os.Exit(1)
	}
	contents, err := os.ReadFile(loc)
	if err != nil {
		fmt.Printf("error loading runfile message.txt: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s", string(contents))
}
