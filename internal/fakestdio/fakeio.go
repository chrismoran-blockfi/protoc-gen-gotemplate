package fakestdio

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
)

// FakeStdio can be used to fake stdin and capture stdout.
// Between creating a new FakeStdio and calling ReadAndRestore on it,
// code reading os.Stdin will get the contents of stdinText passed to New.
// Output to os.Stdout will be captured and returned from ReadAndRestore.
// FakeStdio is not reusable; don't attempt to use it after calling
// ReadAndRestore, but it should be safe to create a new FakeStdio.
type FakeStdio struct {
	origStdout   *os.File
	stdoutReader *os.File

	inCh  chan bool
	outCh chan []byte

	origStdin   *os.File
	stdinWriter *os.File
}

func New(stdinText []byte) (*FakeStdio, error) {
	// Pipe for stdin.
	//
	//                 ======
	//  w ------------->||||------> r
	// (stdinWriter)   ======      (os.Stdin)
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	// Pipe for stdout.
	//
	//               ======
	//  w ----------->||||------> r
	// (os.Stdout)   ======      (stdoutReader)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origStdin := os.Stdin
	os.Stdin = stdinReader

	inCh := make(chan bool)
	go func() {
		_, err = stdinWriter.Write(stdinText)
		if err != nil {
			_ = stdinWriter.Close()
			os.Stdin = origStdin
			inCh <- false
			return
		}

		inCh <- true
	}()

	origStdout := os.Stdout
	os.Stdout = stdoutWriter

	outCh := make(chan []byte)

	// This goroutine reads stdout into a buffer in the background.
	go func() {
		var b bytes.Buffer
		if _, err := io.Copy(&b, stdoutReader); err != nil {
			log.Println(err)
		}
		outCh <- b.Bytes()
	}()

	return &FakeStdio{
		origStdout:   origStdout,
		stdoutReader: stdoutReader,
		inCh:         inCh,
		outCh:        outCh,
		origStdin:    origStdin,
		stdinWriter:  stdinWriter,
	}, nil
}

// CloseStdin closes the fake stdin. This may be necessary if the process has
// logic for reading stdin until EOF; otherwise such code would block forever.
func (sf *FakeStdio) CloseStdin() {
	if sf.stdinWriter != nil {
		_ = sf.stdinWriter.Close()
		sf.stdinWriter = nil
	}
}

func (sf *FakeStdio) AutoCloseStdin() {
	go func() {
		_ = <-sf.inCh

		sf.CloseStdin()
	}()
}

// ReadAndRestore collects all captured stdout and returns it; it also restores
// os.Stdin and os.Stdout to their original values.
func (sf *FakeStdio) ReadAndRestore() ([]byte, error) {
	if sf.stdoutReader == nil {
		return nil, fmt.Errorf("ReadAndRestore from closed FakeStdio")
	}

	// Close the writer side of the faked stdout pipe. This signals to the
	// background goroutine that it should exit.
	_ = os.Stdout.Close()
	out := <-sf.outCh

	os.Stdout = sf.origStdout
	os.Stdin = sf.origStdin

	if sf.stdoutReader != nil {
		_ = sf.stdoutReader.Close()
		sf.stdoutReader = nil
	}

	if sf.stdinWriter != nil {
		_ = sf.stdinWriter.Close()
		sf.stdinWriter = nil
	}

	return out, nil
}
