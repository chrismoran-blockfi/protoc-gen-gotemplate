package io

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func PeekStdin() ([]byte, error) {
	var data []byte
	var pin, pout *os.File
	var err error
	var written int64

	if pin, pout, err = os.Pipe(); err != nil {
		return nil, err
	}
	if data, err = ioutil.ReadAll(os.Stdin); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(data)
	if written, err = io.Copy(pout, buf); err != nil || written != int64(len(data)) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New(fmt.Sprintf("buffer underwrite: %v != %v", written, len(data)))
	}
	if err = pout.Close(); err != nil {
		return nil, err
	}
	os.Stdin = pin

	return data, nil
}

func ReplaceStdin(data []byte) error {
	pin, pout, _ := os.Pipe()
	buf := bytes.NewBuffer(data)
	if written, err := io.Copy(pout, buf); err != nil || written != int64(len(data)) {
		if err != nil {
			return err
		}
		return errors.New(fmt.Sprintf("buffer underwrite: %v != %v", written, len(data)))
	}
	if err := pout.Close(); err != nil {
		return err
	}
	os.Stdin = pin

	return nil
}
