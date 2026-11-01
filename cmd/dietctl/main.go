package main

import (
	"os"

	"github.com/dfgoodfellow2/diet-tracker/v2/ui/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
