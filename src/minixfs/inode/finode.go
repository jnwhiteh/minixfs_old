package inode

import (
	"io"
	"log"
	. "minixfs/common"
	"sync"
)

// The interface to the dinode is returned as a channel, with methods wrapping
// the channel operations.
type finodeRequestChan struct {
	in    chan m_finode_req
	out   chan m_finode_res
	inode *cacheInode
}

// A Finode is a process-oriented file inode, shared amongst all open
// instances of the file represented by this inode. File system operations
// that do not affect this file must not block a read() call to this file.
// Additionally, no read operations on this file should block an independent
// read() call for this file. In particular, open/close operations must not
// block reads, and multiple independent read requests must be allowed.
type finode struct {
	inode   *cacheInode
	devinfo DeviceInfo
	cache   BlockCache

	in     chan m_finode_req
	out    chan m_finode_res
	locked chan m_finode_req

	waitGroup *sync.WaitGroup // used for mutual exclusion for writes
	closed    chan bool
}

func NewFinodeServer(inode *cacheInode) Finode {
	finode := &finode{
		inode,
		inode.devinfo,
		inode.bcache,
		make(chan m_finode_req),
		make(chan m_finode_res),
		nil,
		new(sync.WaitGroup),
		nil,
	}

	go finode.loop()

	return &finodeRequestChan{
		finode.in,
		finode.out,
		inode,
	}
}

func (fi *finode) loop() {
	var in <-chan m_finode_req = fi.in
	var out chan<- m_finode_res = fi.out

	for {
		req, ok := <-in
		if !ok {
			return
		}

		switch req := req.(type) {
		case m_finode_req_lock:
			newin := make(chan m_finode_req)
			in = newin
			out <- m_finode_res_lock{&finodeRequestChan{
				newin,
				fi.out,
				fi.inode,
			}}
		case m_finode_req_unlock:
			in = fi.in
			out <- m_finode_res_unlock{true}
		case m_finode_req_truncate:
			rip := fi.inode
			Truncate(rip, rip.bitmap, rip.bcache)
			// TODO: This should probably be capable of erroring
			out <- m_finode_res_truncate{nil}
		case m_finode_req_read:
			fi.waitGroup.Add(1)

			callback := make(chan m_finode_res_io)
			out <- m_finode_res_asyncio{callback}

			// Launch a new goroutine to perform the read, using the callback
			// channel to return the result.
			go func() {
				defer close(callback)
				defer fi.waitGroup.Done()
				n, err := fi.read(req.buf, req.pos)
				callback <- m_finode_res_io{n, err}
			}()
		case m_finode_req_write:
			// Wait for any outstanding read requests to finish
			fi.waitGroup.Wait()

			n, err := fi.write(req.buf, req.pos)
			out <- m_finode_res_io{n, err}
		case m_finode_req_close:
			fi.waitGroup.Wait()
			out <- m_finode_res_err{nil}
			close(fi.in)
			close(fi.out)
		}
	}

	if fi.closed != nil {
		fi.closed <- true
	}
}

//////////////////////////////////////////////////////////////////////////////
// Public interface
//////////////////////////////////////////////////////////////////////////////

func (fi *finodeRequestChan) Devnum() int {
	return fi.inode.Devnum()
}

func (fi *finodeRequestChan) Inum() int {
	return fi.inode.Inum()
}

// Read up to len(b) bytes from the file from position 'pos'
func (fi *finodeRequestChan) Read(b []byte, pos int) (int, error) {
	fi.in <- m_finode_req_read{b, pos}
	ares := (<-fi.out).(m_finode_res_asyncio)
	res := (<-ares.callback)
	return res.n, res.err
}

// Write len(b) bytes to the file at position 'pos'
func (fi *finodeRequestChan) Write(data []byte, pos int) (n int, err error) {
	fi.in <- m_finode_req_write{data, pos}
	res := (<-fi.out).(m_finode_res_io)
	return res.n, res.err
}

// Close an instance of this finode.
func (fi *finodeRequestChan) Close() error {
	fi.in <- m_finode_req_close{}
	res := (<-fi.out).(m_finode_res_err)
	return res.err
}

func (fi *finodeRequestChan) Lock() Finode {
	fi.in <- m_finode_req_lock{}
	res := (<-fi.out).(m_finode_res_lock)
	return res.finode
}

func (fi *finodeRequestChan) Unlock() {
	fi.in <- m_finode_req_unlock{}
	res := (<-fi.out).(m_finode_res_unlock)
	if !res.ok {
		panic("Attempt to unlock a non-locked Finode")
	}
}

func (fi *finodeRequestChan) Truncate() error {
	fi.in <- m_finode_req_truncate{}
	res := (<-fi.out).(m_finode_res_truncate)
	return res.err
}

// FIXME: All of these
func (fi *finodeRequestChan) IsMountPoint() bool {
	return fi.inode.mount
}

func (fi *finodeRequestChan) SetMountPoint(mounted bool) {
	fi.inode.mount = mounted
}

func (fi *finodeRequestChan) Links() int {
	return int(fi.inode.disk.Nlinks)
}

func (fi *finodeRequestChan) IncLinks() {
	fi.inode.disk.Nlinks++
}

func (fi *finodeRequestChan) DecLinks() {
	fi.inode.disk.Nlinks--
}

func (fi *finodeRequestChan) SetDirty(dirty bool) {
	fi.inode.dirty = true
}

func (fi *finodeRequestChan) Size() int {
	return int(fi.inode.disk.Size)
}

//////////////////////////////////////////////////////////////////////////////
// Private implementations
//////////////////////////////////////////////////////////////////////////////

func (fi *finode) read(b []byte, pos int) (int, error) {
	// We want to read at most len(b) bytes from the given file. This data
	// will almost certainly be split up amongst multiple blocks.
	curpos := pos

	// Rather than getting fancy, just slice b to contain only enough space
	// for the data that is available
	// TODO: Should this rely on the inode size?
	if curpos+len(b) > int(fi.inode.disk.Size) {
		b = b[:int(fi.inode.disk.Size)-curpos]
	}

	if curpos >= int(fi.inode.disk.Size) {
		return 0, io.EOF
	}

	blocksize := fi.devinfo.Blocksize

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := curpos % blocksize
	bnum := ReadMap(fi.inode, curpos, fi.cache)

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fi.cache.GetBlock(fi.inode.devnum, bnum, FULL_DATA_BLOCK, NORMAL)
	bdata, bok := bp.Block.(FullDataBlock)
	if !bok {
		// TODO: Attempt to read from an invalid location, what should happen?
		return 0, EINVAL
	}

	if len(b) < blocksize-offset { // this block contains all the data we need
		for i := 0; i < len(b); i++ {
			b[i] = bdata[offset+i]
		}
		curpos += len(b)
		fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
		return len(b), nil
	}

	// we need this entire first block, so start filling
	var numBytes int = 0
	for i := 0; i < blocksize-offset; i++ {
		b[i] = bdata[offset+i]
		numBytes++
	}

	fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
	curpos += numBytes

	// At this stage, all reads should be on block boundaries. The final block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = ReadMap(fi.inode, curpos, fi.cache)
		bp := fi.cache.GetBlock(fi.inode.devnum, bnum, FULL_DATA_BLOCK, NORMAL)
		if _, sok := bp.Block.(FullDataBlock); !sok {
			log.Printf("block num: %d", bp.Blockno)
			log.Panicf("When reading block %d for position %d, got IndirectBlock", bnum, curpos)
		}

		bdata = bp.Block.(FullDataBlock)

		bytesLeft := len(b) - numBytes // the number of bytes still needed

		// If we only need a portion of this block
		if bytesLeft < blocksize {

			for i := 0; i < bytesLeft; i++ {
				b[numBytes] = bdata[i]
				numBytes++
			}

			curpos += bytesLeft
			fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		curpos += len(bdata)
		fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
	}

	return numBytes, nil
}

func (fi *finode) write(data []byte, pos int) (n int, err error) {
	// TODO: This implementation is direct and doesn't match the abstractions
	// in the original source. At some point it should be reviewed.
	cum_io := 0
	position := pos
	fsize := int(fi.inode.disk.Size)

	// Check in advance to see if file will grow too big
	if position > fi.devinfo.Maxsize-len(data) {
		return 0, EFBIG
	}

	// Clear the zone containing the current present EOF if hole about to be
	// created. This is necessary because all unwritten blocks prior to the
	// EOF must read as zeros.
	if position > fsize {
		ClearZone(fi.inode, fsize, 0, fi.cache)
	}

	bsize := fi.devinfo.Blocksize
	nbytes := len(data)
	// Split the transfer into chunks that don't span two blocks.
	for nbytes != 0 {
		off := (position % bsize)
		var min int
		if nbytes < bsize-off {
			min = nbytes
		} else {
			min = bsize - off
		}
		chunk := min
		if chunk < 0 {
			chunk = bsize - off
		}

		// Read or write 'chunk' bytes, fetch the first block
		err = WriteChunk(fi.inode, position, off, chunk, data, fi.cache)
		if err != nil {
			break // EOF reached
		}

		// Update counters and pointers
		data = data[chunk:] // user buffer
		nbytes -= chunk     // bytes yet to be written
		cum_io += chunk     // bytes written so far
		position += chunk   // position within the file
	}

	itype := fi.inode.disk.Mode & I_TYPE
	if itype == I_REGULAR || itype == I_DIRECTORY {
		if position > fsize {
			fi.inode.disk.Size = int32(position)
		}
	}

	// TODO: Update times
	if err == nil {
		fi.inode.dirty = true
	}

	return cum_io, err
}

var _ Finode = &finodeRequestChan{}
