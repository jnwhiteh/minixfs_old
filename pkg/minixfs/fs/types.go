package fs

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"os"
)

type fileSystem interface {
	Shutdown() os.Error
	Mount(dev RandDevice, path string) os.Error
	Unmount(dev RandDevice) os.Error
	Spawn(pid int, umask uint16, rootpath string) (*Process, os.Error)
	Exit(proc *Process)
	Open(proc *Process, path string, flags int, mode uint16) (*File, os.Error)
	Close(proc *Process, file *File) os.Error
	Unlink(proc *Process, path string) os.Error
	Mkdir(proc *Process, path string, mode uint16) os.Error
	Rmdir(proc *Process, path string) os.Error
	Chdir(proc *Process, path string) os.Error

	Seek(proc *Process, file *File, pos, whence int) (int, os.Error)
	Read(proc *Process, file *File, b []byte) (int, os.Error)
	Write(proc *Process, file *File, b []byte) (int, os.Error)
}

type filp struct {
	mode  uint16
	flags int
	inode *CacheInode
	count int
	pos   int
}

type Process struct {
	pid     int         // the numeric id of this process
	umask   uint16      // file creation mask
	rootdir *CacheInode // root directory of the process
	workdir *CacheInode // working directory of the process
	filp    []*filp     // the list of file descriptors
}

type File struct {

}