//go:debug x509negativeserial=1
package main

import (
	"os"

	"ubnt-exporter/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
