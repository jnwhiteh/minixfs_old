package fs

import (
	"bytes"
	. "minixfs/common"
	. "minixfs/testutils"
	"os"
	"testing"
)

// Test write functionality by taking the data from the host system and
// writing it to the guest system along with the host system (at the same
// time) and comparing the returns from these functions. Then compare the
// written data to the original data to make sure it was written correctly.
func TestWrite(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	ofile, err := os.OpenFile("../../../europarl-en.txt", os.O_RDONLY, 0666)
	if err != nil {
		FatalHere(test, "Could not open original file: %s", err)
	}
	if ofile.Close() != nil {
		FatalHere(test, "Failed when closing original file: %s", err)
	}

	// Read the data for the entire file
	filesize := 4489799 // known
	filedata := make([]byte, filesize)

	// Open the two files that will be written to
	gfile, err := fs.Open(proc, "/tmp/europarl-en.txt", O_CREAT|O_TRUNC|O_RDWR, 0666)
	if err != nil || gfile == nil {
		FatalHere(test, "Could not open file on guest: %s", err)
	}
	hfile, err := os.OpenFile("/tmp/europarl-en.txt", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil || hfile == nil {
		FatalHere(test, "Could not open file on host: %s", err)
	}

	// Write the data to a file on the host/guest operating systems in sync
	blocksize := fs.devinfo[ROOT_DEVICE].Blocksize
	numbytes := blocksize + (blocksize / 3)
	pos := 0

	for pos < filesize {
		// Write the next numbytes bytes to the file
		endpos := pos + numbytes
		if endpos > filesize {
			endpos = filesize
		}
		data := filedata[pos:endpos]

		gn, gerr := fs.Write(proc, gfile, data)
		hn, herr := hfile.Write(data)

		if gn != hn {
			ErrorHere(test, "Bytes read mismatch at offset %d: expected %d, got %d", pos, hn, gn)
		}
		if gerr != herr {
			ErrorHere(test, "Error mismatch at offset %d: expected '%s', got '%s'", pos, herr, gerr)
		}

		pos += gn
	}

	if hfile.Close() != nil {
		ErrorHere(test, "Failed when closing host file")
	}

	// Seek to beginning of file
	fs.Seek(proc, gfile, 0, 0)
	written := make([]byte, filesize)
	n, err := fs.Read(proc, gfile, written)
	if n != filesize {
		ErrorHere(test, "Verify count mismatch expected %d, got %d", filesize, n)
	}
	if err != nil {
		ErrorHere(test, "Error when reading to verify: %s", err)
	}

	compare := bytes.Compare(filedata, written)
	if compare != 0 {
		ErrorHere(test, "Error comparing written data expected %d, got %d", 0, compare)
	}

	if fs.Close(proc, gfile) != nil {
		ErrorHere(test, "Error when closing out the written file: %s", err)
	}

	err = fs.Unlink(proc, "/tmp/europarl-en.txt")
	if err != nil {
		ErrorHere(test, "Failed when unlinking written file: %s", err)
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
