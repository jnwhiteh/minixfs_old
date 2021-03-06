package common

type BlockType int

const (
	CHAR_BIT    = 8 // number of bits in a char
	START_BLOCK = 2 // first block of FS (not counting SB)

	ROOT_DEVICE  = 0 // the root device number
	ROOT_INODE   = 1 // the root inode number
	ROOT_PROCESS = 0 // the root process number (dummy)

	NR_FILPS   = 128 // # slots in filp table
	NR_INODES  = 64  // # slots in "in core" inode table
	NR_PROCS   = 32  // # slots in the process table
	NR_DEVICES = 8   // # slots in the devices/bitmap tables

	OPEN_MAX = 20 // the maximum number of files that can be opened by a process
	NAME_MAX = 60 // the maximum size of a filename

	// The buffer cache should be made as large as you can afford
	NR_BUFS     = 1280            // # blocks in the buffer cache
	NR_BUF_HASH = 2048            // size of buf hash table; MUST BE POWER OF 2
	HASH_MASK   = NR_BUF_HASH - 1 // mask for hashing block numbers

	SUPER_V3 = 0x4d5a

	V2_INODE_SIZE  = 64 // the size of an inode in bytes
	V2_DIRENT_SIZE = 64 // the size of a dirent in bytes
	DIR_ENTRY_SIZE = V2_DIRENT_SIZE

	V2_NR_DZONES     = 7  // number of direct zones in a V2 inode
	V2_NR_TZONES     = 10 // total # of zone numbers in a V2 inode
	V2_ZONE_NUM_SIZE = 4  // the number of bytes in a zone_t (uint32)

	ZONE_SHIFT = 0 // unused, but leaving in for clarity

	IMAP = 0 // operations are on the inode bitmap
	ZMAP = 1 // operations are on the zone bitmap

	NO_ZONE        = 0
	NO_BLOCK       = 0
	NO_BIT         = 0
	NO_DEV         = -1
	NO_FILE        = -1
	NO_INODE       = -1
	RESERVED_INODE = -2

	// When a block is released, the type of usage is passed to put_block()
	WRITE_IMMED = 0100 // block should be written to disk now
	ONE_SHOT    = 0200 // set if block not likely to be needed soon

	I_TYPE          = 0170000 // bit mask for type of inode
	I_UNIX_SOCKET   = 0140000 // unix domain socket
	I_SYMBOLIC_LINK = 0120000 // file is a symbolic link
	I_REGULAR       = 0100000 // regular file, not dir or special
	I_BLOCK_SPECIAL = 0060000 // block special file
	I_DIRECTORY     = 0040000 // file is a directory
	I_CHAR_SPECIAL  = 0020000 // character special file
	I_NAMED_PIPE    = 0010000 // named pipe (FIFO)
	I_SET_UID_BIT   = 0004000 // set effective uid_t on exec
	I_SET_GID_BIT   = 0002000 // set effective gid_t on exec
	I_SET_STCKY_BIT = 0001000 // sticky bit

	ALL_MODES = 0007777 // all bits for user, group and others
	RWX_MODES = 0000777 // mode bits for RWX only

	R_BIT       = 0000004 // Rwx protection bit
	W_BIT       = 0000002 // rWx protection bit
	X_BIT       = 0000001 // rwX protection bit
	I_NOT_ALLOC = 0000000 // this inode is free

	// Oflag values for open().  POSIX Table 6-4.
	O_CREAT  = 00100 // creat flag if it doesn't exist
	O_EXCL   = 00200 // exclusive use flag
	O_NOCTTY = 00400 // do not assign a controlling terminal
	O_TRUNC  = 01000 // truncate flag

	// File status flags for open() and fcntl().  POSIX Table 6-5.
	O_APPEND   = 02000  // set append mode
	O_NONBLOCK = 04000  // no delay
	O_REOPEN   = 010000 // automaticalloy re-open device after driver restart?

	// File access modes for open() and fcntl().  POSIX Table 6-6.
	O_RDONLY = 0 // open(name, O_RDONLY) opens read only
	O_WRONLY = 1 // open(name, O_WRONLY) opens write only
	O_RDWR   = 2 // open(name, O_RDWR) opens read/write

	// Mask for use with file access modes. POSIX Table 6-7.
	O_ACCMODE = 03 /* mask for file access modes */

	NORMAL   = 0 // forces get_block to do disk read
	NO_READ  = 1 // prevents get_block from doing disk read
	PREFETCH = 2 // tells get_block not to read or mark dev

	READING = 0 // copy data from user
	WRITING = 1 // copy data to user

	INODE_BLOCK        BlockType = 0 // inode block
	DIRECTORY_BLOCK    BlockType = 1 // directory block
	INDIRECT_BLOCK     BlockType = 2 // pointer block
	MAP_BLOCK          BlockType = 3 // bit map
	FULL_DATA_BLOCK    BlockType = 5 // data, fully used
	PARTIAL_DATA_BLOCK BlockType = 6 // data, partly used
)
