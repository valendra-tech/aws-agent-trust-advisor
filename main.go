package main

import (
	"os"

	"github.com/valendra-tech/aws-agent-trust-advisor/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
