package fs

import (
	"encoding/binary"
	"log"
	"minixfs/bcache"
	"minixfs/bitmap"
	. "minixfs/common"
	"minixfs/device"
	"minixfs/inode"
	"sync"
)

// FileSystem encapsulates a MINIX file system.
type FileSystem struct {
	devices   []BlockDevice // the block devices that comprise the file system
	devinfo   []DeviceInfo  // the geometry/params for the given device
	mountinfo []mountInfo   // the information about the mount points
	bitmaps   []Bitmap      // the bitmaps for the given devices

	bcache BlockCache // the block cache (shared across all devices)
	icache InodeCache // the inode cache (shared across all devices)

	filps []*Filp    // the filep (file position) table
	procs []*Process // an array of processes that have been spawned

	// Locks protecting the above slices
	m struct {
		device *sync.RWMutex
		filp   *sync.RWMutex
		proc   *sync.RWMutex
	}
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, error) {
	dev, err := device.NewFileDevice(filename, binary.LittleEndian)

	if err != nil {
		return nil, err
	}

	return NewFileSystem(dev)
}

// Create a new FileSystem from a given file on the filesystem
func NewFileSystem(dev BlockDevice) (*FileSystem, error) {
	fs := new(FileSystem)

	fs.devices = make([]BlockDevice, NR_DEVICES)
	fs.devinfo = make([]DeviceInfo, NR_DEVICES)
	fs.mountinfo = make([]mountInfo, NR_DEVICES)
	fs.bitmaps = make([]Bitmap, NR_DEVICES)

	devinfo, err := GetDeviceInfo(dev)
	if err != nil {
		return nil, err
	}

	fs.bcache = bcache.NewLRUCache(NR_DEVICES, NR_BUFS, NR_BUF_HASH)
	fs.icache = inode.NewCache(fs.bcache, NR_DEVICES, NR_INODES)

	fs.devices[ROOT_DEVICE] = dev
	fs.devinfo[ROOT_DEVICE] = devinfo
	fs.bitmaps[ROOT_DEVICE] = bitmap.NewBitmap(devinfo, fs.bcache, ROOT_DEVICE)

	fs.filps = make([]*Filp, NR_INODES)
	fs.procs = make([]*Process, NR_PROCS)

	if err := fs.bcache.MountDevice(ROOT_DEVICE, dev, devinfo); err != nil {
		log.Printf("Could not mount root device: %s", err)
		return nil, err
	}
	fs.icache.MountDevice(ROOT_DEVICE, fs.bitmaps[ROOT_DEVICE], devinfo)

	// Fetch the root inode
	rip, err := fs.icache.GetInode(ROOT_DEVICE, ROOT_INODE)
	if err != nil {
		log.Printf("Failed to fetch root inode: %s", err)
		return nil, err
	}

	// Create the root process
	fs.procs[ROOT_PROCESS] = &Process{ROOT_PROCESS, 022, rip, rip,
		make([]*Filp, OPEN_MAX),
		make([]*File, OPEN_MAX),
		new(sync.Mutex),
	}

	fs.m.device = new(sync.RWMutex)
	fs.m.filp = new(sync.RWMutex)
	fs.m.proc = new(sync.RWMutex)

	return fs, nil
}

//var _ fileSystem = FileSystem{}
