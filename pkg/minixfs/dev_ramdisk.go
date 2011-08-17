package minixfs

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"sync"
)

// Need to implement a io.ReadWriter so this can be used with binary/encoding.
// This means implementing a stateful Read/Write, and we can just sub-slice in
// order to 'seek' to the right position.
type bytestore []byte

var _ io.Reader = bytestore(nil)
var _ io.Writer = bytestore(nil)

func (b bytestore) Read(p []byte) (n int, err os.Error) {
	if len(p) > len(b) {
		err = ENOENT
	}
	n = len(b)
	copy(p, b)
	return
}

func (b bytestore) Write(p []byte) (n int, err os.Error) {
	if len(p) > len(b) {
		err = ENOENT
		n = len(b)
		copy(b, p)
	} else {
		n = len(p)
		copy(b, p)
	}
	return
}

type ramdiskDevice struct {
	data  bytestore
	in    chan BlockRequest       // channel on which to receive requests
	out   chan chan BlockResponse // channel via which callback channels are delivered
	rwait *sync.WaitGroup         // a waitgroup used to 
}

func NewRamdiskDevice(data []byte) (BlockDevice, os.Error) {
	dev := &ramdiskDevice{
		data,
		make(chan BlockRequest),
		make(chan chan BlockResponse),
		new(sync.WaitGroup),
	}

	go dev.loop()
	return dev, nil
}

func NewRamdiskDeviceFile(filename string) (BlockDevice, os.Error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	err = file.Close()
	if err != nil {
		return nil, err
	}
	return NewRamdiskDevice(data)
}

func (dev *ramdiskDevice) loop() {
	var in <-chan BlockRequest = dev.in
	var out chan<- chan BlockResponse = dev.out

	for req := range in {
		callback := make(chan BlockResponse)
		out <- callback

		switch req.call {
		case DEV_READ:
			// device.Read
			dev.rwait.Add(1)

			// Launch a new goroutine to perform the read, using the callback
			// channel to return the result.
			go func() {
				defer close(callback)
				defer dev.rwait.Done()
				if req.pos > int64(len(dev.data)) {
					callback <- BlockResponse{ERR_SEEK}
					return
				}
				sub := dev.data[req.pos:]
				err := binary.Read(sub, binary.LittleEndian, req.buf)
				callback <- BlockResponse{err}
			}()
		case DEV_WRITE:
			// device.Write
			// wait for any reading goroutines to finish, blocking so no more
			// can start.
			dev.rwait.Wait()

			if req.pos > int64(len(dev.data)) {
				callback <- BlockResponse{ERR_SEEK}
				return
			} else {
				sub := dev.data[req.pos:]
				err := binary.Write(sub, binary.LittleEndian, req.buf)
				callback <- BlockResponse{err}
			}
			close(callback)
		case DEV_CLOSE:
			// device.Close
			dev.data = nil
			callback <- BlockResponse{nil}
			close(callback)
			close(dev.in)
			close(dev.out)
		default:
			callback <- BlockResponse{ERR_BADCALL}
			close(callback)
		}
	}
}

func (dev *ramdiskDevice) Read(buf interface{}, pos int64) os.Error {
	dev.in <- BlockRequest{DEV_READ, buf, pos}
	cback := <-dev.out
	res := <-cback
	return res.err
}

func (dev *ramdiskDevice) Write(buf interface{}, pos int64) os.Error {
	dev.in <- BlockRequest{DEV_WRITE, buf, pos}
	cback := <-dev.out
	res := <-cback
	return res.err
}

func (dev *ramdiskDevice) Close() os.Error {
	dev.in <- BlockRequest{DEV_CLOSE, nil, 0}
	cback := <-dev.out
	res := <-cback
	return res.err
}