package minixfs

import "log"
import "os"

type Inode struct {
	*disk_inode
	dev   int
	count uint
	inum  uint
	dirty bool
}

func (inode *Inode) GetType() uint16 {
	return inode.Mode & I_TYPE
}

func (inode *Inode) IsDirectory() bool {
	return inode.GetType() == I_DIRECTORY
}

func (inode *Inode) IsRegular() bool {
	return inode.GetType() == I_REGULAR
}

func (inode *Inode) Inum() uint {
	return inode.inum
}

// Retrieve an Inode from disk/cache given an Inode number. The 0th Inode
// is reserved and unallocatable, so we return an error when it is requested
// The root inode on the disk is ROOT_INODE_NUM, and should be located 64
// bytes into the first block following the bitmaps.

func (fs *FileSystem) get_inode(dev int, num uint) (*Inode, os.Error) {
	if num == 0 {
		return nil, os.NewError("Invalid inode number")
	}

	avail := -1
	for i := 0; i < NR_INODES; i++ {
		rip := fs.inodes[i]
		if rip != nil && rip.count > 0 { // only check used slots for (dev, numb)
			if int(rip.dev) == dev && rip.inum == num {
				// this is the inode that we are looking for
				rip.count++
				return rip, nil
			}
		} else {
			avail = i // remember this free slot for late
		}
	}

	// Inode we want is not currently in ise. Did we find a free slot?
	if avail == -1 { // inode table completely full
		return nil, ENFILE
	}

	// A free inode slot has been located. Load the inode into it
	xp := fs.inodes[avail]
	if xp == nil {
		xp = new(Inode)
	}

	super := fs.supers[dev]

	// For a 4096 block size, inodes 0-63 reside in the first block
	block_offset := super.Imap_blocks + super.Zmap_blocks + 2
	block_num := ((num - 1) / super.inodes_per_block) + uint(block_offset)

	// Load the inode from the disk and create in-memory version of it
	bp := fs.get_block(dev, int(block_num), INODE_BLOCK)
	inodeb := bp.block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[(num-1)%super.inodes_per_block]
	xp.disk_inode = inode_d
	xp.dev = dev
	xp.inum = num
	xp.count = 1
	xp.dirty = false

	// add the inode to the cache
	fs.inodes[avail] = xp

	return xp, nil
}

// Allocate a free inode on the given device and return a pointer to it.
func (fs *FileSystem) alloc_inode(dev int, mode uint16) *Inode {
	super := fs.supers[dev]

	// Acquire an inode from the bit map
	b := fs.alloc_bit(dev, IMAP, super.I_Search)
	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return nil
	}

	super.I_Search = b

	// Try to acquire a slot in the inode table
	inode, err := fs.get_inode(dev, b)
	if err != nil {
		log.Printf("Failed to get inode: %d", b)
		return nil
	}

	inode.Mode = mode
	inode.Nlinks = 0
	inode.Uid = 0 // TODO: Must get the current uid
	inode.Gid = 0 // TODO: Must get the current gid

	fs.wipe_inode(inode)
	return inode
}

// Return an inode to the pool of free inodes
func (fs *FileSystem) free_inode(inode *Inode) {
	super := fs.supers[inode.dev]

	fs.free_bit(inode.dev, IMAP, inode.inum)
	if inode.inum < super.I_Search {
		super.I_Search = inode.inum
	}
}

func (fs *FileSystem) wipe_inode(inode *Inode) {
	inode.Size = 0
	// TODO: Update ATIME, CTIME, MTIME
	inode.dirty = true
	inode.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode.Zone[i] = NO_ZONE
	}
}

func (fs *FileSystem) dup_inode(inode *Inode) {
	inode.count++
}

// The caller is no longer using this inode. If no one else is using it
// either write it back to the disk immediately. If it has no links,
// truncate it and return it to the pool of available inodes.
func (fs *FileSystem) put_inode(rip *Inode) {
	if rip == nil {
		return
	}
	rip.count--
	if rip.count == 0 { // means no one is using it now
		if rip.Nlinks == 0 { // free the inode
			fs.truncate(rip) // return all the disk blocks
			rip.Mode = I_NOT_ALLOC
			rip.dirty = true
			fs.free_inode(rip)
		} else {
			// TODO: Handle the pipe case here
			// if rip.pipe == true {
			//   truncate(rip)
			// }
		}
		// rip.pipe = false
		if rip.dirty {
			// TODO: Implement RWInode, which will write the inode block back
			//fs.RWInode(rip, WRITING)
		}
	}
}
