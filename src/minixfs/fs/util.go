package fs

import (
	"log"
	"math"
	. "minixfs/common"
	"path/filepath"
	"strings"
)

// Unmount a device (must be called under the fs.m.device mutex)
func (fs *FileSystem) unmount(devno int) error {
	if fs.devices[devno] == nil {
		return EINVAL
	}

	// See if the mounted device is busy. Only one inode using it should be
	// open, the root inode, and only once.
	if fs.icache.IsDeviceBusy(devno) {
		return EBUSY // can't unmount a busy file system
	}

	minfo := fs.mountinfo[devno]
	if minfo.imount != nil {
		rip := minfo.imount
		if rip.IsDirectory() {
			dinode := rip.Dinode()
			dinode.SetMountPoint(false) // inode returns to normal
		} else if rip.IsRegular() {
			finode := rip.Finode()
			finode.SetMountPoint(false) // inode returns to normal
		}
		fs.icache.PutInode(minfo.imount) // release the inode mounted on
	}
	if minfo.isup != nil {
		fs.icache.PutInode(minfo.isup) // release the root inode of the mounted fs
	}

	// Flush and invalidate the cache
	fs.bcache.Flush(devno)
	fs.bcache.Invalidate(devno)

	// Close the device the file system lives on
	if err := fs.devices[devno].Close(); err != nil {
		log.Printf("Error when closing device %d: %s", devno, err)
	}

	// Shut down the bitmap process
	if err := fs.bitmaps[devno].Close(); err != nil {
		log.Printf("Error when closing bitmap %d: %s", devno, err)
	}

	fs.devices[devno] = nil
	fs.bitmaps[devno] = nil
	fs.devinfo[devno] = NO_DEVINFO
	return nil
}

func (fs *FileSystem) close(proc *Process, file *File) error {
	if file.fd == NO_FILE {
		return EBADF
	}

	fs.icache.PutInode(file.inode)
	proc.filp[file.fd] = nil
	proc.files[file.fd] = nil

	file.Filp.count--
	if file.Filp.count == 0 {
		fs.filps[file.fd] = nil
	}
	file.fd = NO_FILE
	return nil
}

// Allocate a new inode, making a directory entry for it on the path 'path. If
// successful, the parent directory is returned, along with the new node
// itself, and an nil error.
func (fs *FileSystem) newNode(proc *Process, path string, bits uint16, z0 uint) (CacheInode, CacheInode, string, error) {
	// See if the path can be opened down to the last directory
	dirp, rlast, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, nil, "", err
	}

	dinode := dirp.Dinode()

	if dinode.Links() >= math.MaxUint16 {
		fs.icache.PutInode(dirp)
		return nil, nil, "", EMLINK
	}

	// The final directory is accessible. Get the final component of the path
	rip, err := fs.advance(proc, dirp, rlast)
	if rip == nil && err == ENOENT {
		devnum := dirp.Devnum()
		// Last component does not exist. Make new directory entry
		var inum int // this is here to fix shadowing of err
		inum, err = fs.bitmaps[devnum].AllocInode()
		// TODO: Get the current uid/gid
		rip, err = fs.icache.NewInode(devnum, inum, bits, 1, 0, 0, uint32(z0))

		if rip == nil {
			// Can't create new inode, out of inodes
			fs.icache.PutInode(dirp)
			return nil, nil, "", ENFILE
		}

		// Force the inode to disk before making a directory entry to make the
		// system more robust in the face of a crash: an inode with no
		// directory entry is much better than the opposite.
		fs.icache.FlushInode(rip)

		// New inode acquired. Try to make directory entry.
		dinode := dirp.Dinode()
		err = dinode.Link(rlast, inum)

		if err != nil {
			fs.icache.PutInode(dirp)

			var wrino VolatileInode

			if rip.IsDirectory() {
				wrino = rip.Dinode()
			} else {
				wrino = rip.Finode()
			}

			wrino.DecLinks()        // pity, have to free disk inode
			wrino.SetDirty(true)    // dirty inodes are written out
			fs.icache.PutInode(rip) // this call frees the inode
			return nil, nil, "", err
		}
	} else {
		// Either last component exists or there is some problem
		if rip != nil {
			err = EEXIST
		}
	}

	// We now return the parent directory inode, so don't put it here
	return dirp, rip, rlast, err
}

func (fs *FileSystem) eatPath(proc *Process, path string) (CacheInode, error) {
	ldip, rest, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, err // could not open final directory
	}

	// If there is no more path to go, return
	if len(rest) == 0 {
		return ldip, nil
	}

	// Get final component of the path
	rip, err := fs.advance(proc, ldip, rest)
	fs.icache.PutInode(ldip)
	return rip, err
}

func (fs *FileSystem) wipeInode(rip CacheInode) {
	// NYI
}

// TODO: Remove this function?
func (fs *FileSystem) dupInode(rip CacheInode) {
	// Increment the count
	_, _ = fs.icache.GetInode(rip.Devnum(), rip.Inum())
}

func (fs *FileSystem) lastDir(proc *Process, path string) (CacheInode, string, error) {
	path = filepath.Clean(path)

	var rip CacheInode
	if filepath.IsAbs(path) {
		rip = proc.rootdir
	} else {
		rip = proc.workdir
	}

	dinode := rip.Dinode()
	dinode = dinode.Lock()

	// If directory has been removed or path is empty, return ENOENT
	if dinode.Links() == 0 || len(path) == 0 {
		dinode.Unlock()
		return nil, "", ENOENT
	}

	fs.dupInode(rip) // inode will be returned with put_inode
	dinode.Unlock()

	var pathlist []string
	if filepath.IsAbs(path) {
		pathlist = strings.Split(path, string(filepath.Separator))
		pathlist = pathlist[1:]
	} else {
		pathlist = strings.Split(path, string(filepath.Separator))
	}

	for i := 0; i < len(pathlist)-1; i++ {
		newip, _ := fs.advance(proc, rip, pathlist[i])
		fs.icache.PutInode(rip)
		if newip == nil {
			return nil, "", ENOENT
		}
		rip = newip
	}

	if rip.Type() != I_DIRECTORY {
		// last file of path prefix is not a directory
		fs.icache.PutInode(rip)
		return nil, "", ENOTDIR
	}

	return rip, pathlist[len(pathlist)-1], nil
}

func (fs *FileSystem) advance(proc *Process, dirp CacheInode, path string) (CacheInode, error) {
	// if there is no path, just return this inode
	if len(path) == 0 {
		return fs.icache.GetInode(dirp.Devnum(), dirp.Inum())
	}

	// check for a nil inode
	if dirp == nil {
		return nil, nil // TODO: This should return something
	}

	// don't go beyond the current root directory, ever
	if dirp == proc.rootdir && path == ".." {
		return fs.icache.GetInode(dirp.Devnum(), dirp.Inum())
	}

	// If 'path' is not present in the directory, signal error
	dinode := dirp.Dinode()

	// Find and retrieve the component in the directory, if present.
	rip, err := dinode.LookupGet(path, fs.icache)
	if err != nil {
		return nil, ENOENT
	}

	if rip.Inum() == ROOT_INODE {
		if dirp.Inum() == ROOT_INODE {
			// TODO: What does this do?
			if path[1] == '.' {
				if fs.devices[rip.Devnum()] != nil {
					// we can skip the superblock search here since we know
					// that 'i' is the device that we're looking at.
					mountinfo := fs.mountinfo[rip.Devnum()]
					fs.icache.PutInode(rip)
					mnt_dev := mountinfo.imount.Devnum()
					inumb := mountinfo.imount.Inum()
					rip2, _ := fs.icache.GetInode(mnt_dev, inumb) // TODO: ignore error
					rip, _ = fs.advance(proc, rip2, path)
					fs.icache.PutInode(rip2)
				}
			}
		}
	}

	if rip == nil {
		return nil, nil // TODO: Error here?
	}

	// See if the inode is mounted on. If so, switch to the root directory of
	// the mounted file system. The super_block provides the linkage between
	// the inode mounted on and the root directory of the mounted file system.
	var volino VolatileInode
	dinode = nil
	var finode Finode
	if rip.IsDirectory() {
		dinode = rip.Dinode()
		dinode = dinode.Lock()
		volino = dinode
	} else {
		finode = rip.Finode()
		finode = finode.Lock()
		volino = finode
	}

	if rip != nil && volino.IsMountPoint() {
		// The inode is indeed mounted on
		for i := 0; i < NR_DEVICES; i++ {
			if fs.mountinfo[i].imount == rip {
				// Release the inode mounted on. Replace by the inode of the
				// root inode of the mounted device.
				// FIXME: Without this, something bad happens
				if finode != nil {
					finode.Unlock()
					finode = nil
				}
				if dinode != nil {
					dinode.Unlock()
					dinode = nil
				}
				fs.icache.PutInode(rip)
				rip, _ = fs.icache.GetInode(i, ROOT_INODE) // TODO: ignore error
				break
			}
		}
	}

	if finode != nil {
		finode.Unlock()
		finode = nil
	}
	if dinode != nil {
		dinode.Unlock()
		dinode = nil
	}

	return rip, nil
}

// Given a path, fetch the inode for the parent directory of final entry and
// the inode of the final entry itself. In addition, return the portion of the
// path that is the filename of the final entry, so it can be removed from the
// parent directory, and any error that may have occurred.
func (fs *FileSystem) unlinkPrep(proc *Process, path string) (CacheInode, CacheInode, string, error) {
	// Get the last directory in the path
	rldirp, rest, err := fs.lastDir(proc, path)
	if rldirp == nil {
		return nil, nil, "", err
	}

	// The last directory exists. Does the file also exist?
	rip, err := fs.advance(proc, rldirp, rest)
	if rip == nil {
		return nil, nil, "", err
	}

	// If error, return inode
	if err != nil {
		fs.icache.PutInode(rldirp)
		return nil, nil, "", nil
	}

	// Do not remove a mount point
	if rip.Inum() == ROOT_INODE {
		fs.icache.PutInode(rldirp)
		fs.icache.PutInode(rip)
		return nil, nil, "", EBUSY
	}

	return rldirp, rip, rest, nil
}

func (fs *FileSystem) unlinkFile(dirp, rip CacheInode, filename string) error {
	var err error

	// if rip is not nil, it is used to get access to the inode
	if rip == nil {
		// Search for file in directory and try to get its inode
		pdinode := dirp.Dinode()
		rip, err = pdinode.LookupGet(filename, fs.icache)
		if err != nil {
			return ENOENT
		}
	} else {
		fs.dupInode(rip) // inode will be returned with put_inode
	}

	pdinode := dirp.Dinode()
	err = pdinode.Unlink(filename)
	if err == nil {
		if rip.IsDirectory() {
			dinode := rip.Dinode()
			dinode = dinode.Lock()
			dinode.DecLinks()
			dinode.SetDirty(true)
			// TODO: Update times
			dinode.Unlock()
		} else {
			finode := rip.Finode()
			finode = finode.Lock()
			finode.DecLinks()
			finode.SetDirty(true)
			// TODO: Update times
			finode.Unlock()
		}
	}

	fs.icache.PutInode(rip)
	return err
}
