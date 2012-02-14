package inode

// The inode types provided in this file are wrappers around the inode data
// that is stored on disk. Inodes come in four different forms:
//
// Inode interface:
//   Oly provides the device and inode number of the given inode
// CacheInode interface:
//   Provides read-only access to the non-mutable parameters of the inode
//   data structure, as well as a way to access the file or directory
//   interfaces to the inode.
// Finode interface (file-operations):
//   Provides read/write access to the file operations of an inode, including
//   the ability to read and write data to the file, along with the normal
//   accessors.
// Dinode interface (directory-operations):
//   Provides read/write access to the directory operations of an inode,
//   including the ability to lookup entries in the directory, link new
//   entries in the directory, along with the normal accessors.
//
// Both the Finode and Dinode interfaces provide a way to lock a given inode
// to ensure exclusive access to the data structure. This can be used by
// calling code to ensure that the state of the inode will not change during a
// sequence of operations.

import (
	. "minixfs/common"
)

type cacheInode struct {
	disk    *Disk_Inode
	bitmap  Bitmap
	devinfo DeviceInfo
	bcache  BlockCache
	devnum  int
	inum    int
	count   int
	dirty   bool
	mount   bool
	server  interface{}
}

//////////////////////////////////////////////////////////////////////////////
// Once an Inode has been created, the following parameters cannot be changed,
// so having them available as read-only parameters is safe.
//////////////////////////////////////////////////////////////////////////////
func (ci *cacheInode) Devnum() int {
	return ci.devnum
}

func (ci *cacheInode) Inum() int {
	return ci.inum
}

func (ci *cacheInode) IsBusy() bool {
	return ci.count > 1
}

func (ci *cacheInode) Type() uint16 {
	return ci.disk.Mode & I_TYPE
}

func (ci *cacheInode) IsDirectory() bool {
	return ci.disk.Mode&I_TYPE == I_DIRECTORY
}

func (ci *cacheInode) IsRegular() bool {
	return ci.disk.Mode&I_TYPE == I_REGULAR
}

func (ci *cacheInode) Dinode() Dinode {
	if !ci.IsDirectory() {
		return nil
	}
	if dinode, ok := ci.server.(Dinode); ok {
		return dinode
	}
	return nil
}

func (ci *cacheInode) Finode() Finode {
	if !ci.IsRegular() {
		return nil
	}
	if finode, ok := ci.server.(Finode); ok {
		return finode
	}
	return nil
}

var _ Inode = &cacheInode{}
