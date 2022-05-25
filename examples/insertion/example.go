package main

import (
	"context"
	"fmt"
)
import (
	"bytes"
)

import (
	"errors"
)

import (
	"bufio"
)

// @@protoc_insertion_point(imports)

// doContextThing from file
func doContextThing() {
	_ = context.TODO()
	fmt.Println("Just used 'context.TODO()'")
}
func doBytesThing() {
	_ = bytes.Buffer{}
	fmt.Println("Did bytes.Buffer")
}

func doErrorsThing() {
	_ = errors.New("")
	fmt.Println("Did errors.New")
}

func doScannerThing() {
	_ = bufio.Scanner{}
	fmt.Println("Did bufio.Scanner{}")
}

// @@protoc_insertion_point(functions)

func main() {
	// from file
	doContextThing()
	doBytesThing()
	doErrorsThing()
	doScannerThing()
	// @@protoc_insertion_point(main_body)
}
