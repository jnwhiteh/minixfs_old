package common

import (
	"os"
)

// A random access device
type RandDevice interface {
	Read(buf interface{}, pos int64) os.Error
	Write(buf interface{}, pos int64) os.Error
	Close() os.Error
}

// BlockCache is a thread-safe interface to a block cache, wrapping one or
// more block devices.
type BlockCache interface {
	// Attach a new device to the cache
	MountDevice(devno int, dev RandDevice, info DeviceInfo) os.Error
	// Remove a device from the cache
	UnmountDevice(devno int) os.Error
	// Get a block from the cache
	GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock
	// Release (free) a block back to the cache
	PutBlock(cb *CacheBlock, btype BlockType) os.Error
	// Invalidate all blocks for a given device
	Invalidate(dev int)
	// Flush any dirty blocks for a given device to the device
	Flush(dev int)
	// Close the block cache
	Close() os.Error
}

type Bitmap interface {
	// Allocate a free inode, returning the number of the allocated inode
	AllocInode() (int, os.Error)
	// Allocate a free zone, returning the number of the allocated zone. Start
	// looking at zone number 'zstart' in an attempt to provide contiguous
	// allocation of zones.
	AllocZone(zstart int) (int, os.Error)
	// Free an allocated inode
	FreeInode(inum int)
	// Free an allocated zone
	FreeZone(znum int)
	// Close the bitmap server
	Close() os.Error
}

type InodeCache interface {
	// Update the information about a given device
	MountDevice(devno int, bitmap Bitmap, info DeviceInfo)
	// Get an inode from the given device
	GetInode(devno, inum int) (*CacheInode, os.Error)
	// Return the given inode to the cache. If the inode has been altered and
	// it has no other clients, it should be written to the block cache.
	PutInode(rip *CacheInode)
	// Flush the inode to the block cache, ensuring that it will be written
	// the next time the block cache is flushed.
	FlushInode(rip *CacheInode)
	// Returns whether or not the given device is busy. As non-busy device has
	// exactly one client of the root inode.
	IsDeviceBusy(devno int) bool
	// Close the inode cache
	Close() os.Error
}

type Finode interface {
	// Read up to len(buf) bytes from pos within the file. Return the number
	// of bytes actually read and any error that may have occurred.
	Read(buf []byte, pos int) (int, os.Error)
	// Write len(buf) bytes from buf to the given position in the file. Return
	// the number of bytes actually written and any error that may have
	// occurred.
	Write(buf []byte, pos int) (int, os.Error)
	// Close the finode
	Close() os.Error
}

type Dinode interface {
	// Search the directory for an entry named 'name' and return the
	// devno/inum of the inode, if found.
	Lookup(name string) (bool, int, int)
	// Add an entry 'name' to the directory listing, pointing to the 'inum'
	// inode.
	Link(name string, inum int) os.Error
	// Remove the entry named 'name' from the directory listing.
	Unlink(name string) os.Error
	// close the dinode
	Close() os.Error
}
