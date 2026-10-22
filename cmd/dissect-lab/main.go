// Command dissect-lab is a research CLI over the dissect/wire decoders. It is not part of releases.
package main

import (
	"fmt"
	"io"
	"os"
)

const commandList = "segments, tlv, movement, coverage, join, graph"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one dissect-lab invocation. It returns 0 on success, 2 on a usage error and 1 on a runtime error.
// A failed run writes one line to stderr and nothing to stdout.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "dissect-lab: usage: dissect-lab <%s> <round.rec> [flags]\n", commandList)
		return 2
	}
	var cmd func(*lab, []string) error
	switch args[0] {
	case "segments":
		cmd = (*lab).segments
	case "tlv":
		cmd = (*lab).tlv
	case "movement":
		cmd = (*lab).movement
	case "coverage":
		cmd = (*lab).coverage
	case "join":
		cmd = (*lab).join
	case "graph":
		cmd = (*lab).graph
	default:
		fmt.Fprintf(stderr, "dissect-lab: unknown command %q (want %s)\n", args[0], commandList)
		return 2
	}
	// Output is buffered so that a failure after the first line still leaves stdout empty.
	l := &lab{}
	err := cmd(l, args[1:])
	if err != nil {
		if u, ok := err.(usageError); ok {
			fmt.Fprintf(stderr, "dissect-lab: %s: %s\n", args[0], string(u))
			return 2
		}
		fmt.Fprintf(stderr, "dissect-lab: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(l.out.Bytes()); err != nil {
		fmt.Fprintf(stderr, "dissect-lab: %v\n", err)
		return 1
	}
	return 0
}

// usageError marks an error that exits 2 instead of 1.
type usageError string

func (u usageError) Error() string { return string(u) }
