package minixfs

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

var ENOTDIR = os.NewError("ENOTDIR: not a directory")
var ENOENT = os.NewError("ENOENT: no such file or directory")

// EathPath parses the path 'path' and retrieves the associated inode.
func (fs *FileSystem) EatPath(proc *Process, path string) (*Inode) {
	ldip, rest, err := fs.LastDir(proc, path)
	if err != nil {
		return nil // could not open final directory
	}

	// If there is no more path to go, return
	if len(rest) == 0 {
		return ldip
	}

	// Get final compoennt of the path
	rip := fs.Advance(proc, ldip, rest)
	fs.PutInode(ldip)
	return rip
}

// LastDir parses 'path' as far as the last directory, fetching the inode and
// returning it along with the final portion of the path and any error that
// might have occurred.
func (fs *FileSystem) LastDir(proc *Process, path string) (*Inode, string, os.Error) {
	path = filepath.Clean(path)

	var rip *Inode
	if filepath.IsAbs(path) {
		rip = fs.RootDir
	} else {
		rip = fs.WorkDir
	}

	// If directory has been removed or path is empty, return ENOENT
	if rip.Nlinks == 0 || len(path) == 0 {
		return nil, "", ENOENT
	}

	fs.DupInode(rip) // inode will be returned with put_inode

	var pathlist []string
	if filepath.IsAbs(path) {
		pathlist = strings.Split(path, filepath.SeparatorString, -1)
		pathlist = pathlist[1:]
	} else {
		pathlist = strings.Split(path, filepath.SeparatorString, -1)
	}

	for i := 0; i < len(pathlist) - 1; i++ {
		newip := fs.Advance(proc, rip, pathlist[i])
		fs.PutInode(rip)
		if newip == nil {
			return nil, "", ENOENT
		}
		rip = newip
	}

	return rip, pathlist[len(pathlist)-1], nil
}

// Advance looks up the component 'path' in the directory 'dirp', returning
// the inode.

func (fs *FileSystem) Advance(proc *Process, dirp *Inode, path string) (*Inode) {
	// if there is no path, just return this inode
	if len(path) == 0 {
		rip, _ := fs.GetInode(dirp.Inum())
		return rip
	}

	// check for a nil inode
	if dirp == nil {
		return nil
	}

	// ensure that 'path' is an entry in the directory
	numb, err := fs.SearchDir(dirp, path)
	if err != nil {
		panic("could not find entry")
		return nil
	}

	// don't go beyond the current root directory, ever
	if dirp == proc.rootdir && path == ".." {
		rip, _ := fs.GetInode(dirp.Inum())
		return rip
	}

	// the component has been found in the directory, get the inode
	rip, _ := fs.GetInode(uint(numb))
	if rip == nil {
		return nil
	}

	// TODO: Handle mounted file systems here

	return rip
}

// SearchDir searches for an entry named 'path' in the directory given by
// 'dirp'. This function differs from the minix version.
func (fs *FileSystem) SearchDir(dirp *Inode, path string) (int, os.Error) {
	if dirp.GetType() != I_DIRECTORY {
		return 0, ENOTDIR
	}

	// step through the directory on block at a time
	numEntries := dirp.Size / DIR_ENTRY_SIZE
	for pos := 0; pos < int(dirp.Size); pos += int(fs.Block_size) {
		b := fs.ReadMap(dirp, uint(pos)) // get block number
		bp := fs.GetBlock(int(b), DIRECTORY_BLOCK)
		if bp == nil {
			panic("get_block returned NO_BLOCK")
		}

		dirarr := bp.block.(DirectoryBlock)
		for i := 0; i < len(dirarr) && numEntries > 0; i++ {
			if dirarr[i].HasName(path) {
				log.Printf("Found entry %s in inode %d at inode %d", dirarr[i].Name, dirp.inum, dirarr[i].Inum)
				return int(dirarr[i].Inum), nil
			}
			numEntries--
		}
	}

	return 0, ENOENT
}
