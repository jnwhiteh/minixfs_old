package minixfs

import (
	"os"
	"testing"
)

func TestAdvance(test *testing.T) {
	fs, proc := OpenMinix3(test)

	// Helper function to ensure filesystem gets cleaned up properly
	advance := func(proc *Process, rip *CacheInode, path string) (*CacheInode, os.Error) {
		inode, err := fs.advance(proc, rip, path)
		fs.put_inode(rip)
		return inode, err
	}

	var dirp *CacheInode
	var rip *CacheInode
	var err os.Error

	// Advance to /root/.ssh/known_hosts at inode 541
	if dirp, err = fs.advance(proc, proc.rootdir, "root"); dirp == nil {
		test.Logf("Failed to get /root directory inode: %s", err)
		test.FailNow()
	}
	if dirp.inum != 519 {
		test.Errorf("Inodes did not match, expected %d, got %d", 519, dirp.inum)
	}
	// Advance to /root/.ssh
	if dirp, err = advance(proc, dirp, ".ssh"); dirp == nil {
		test.Logf("Failed to get /root/.ssh directory inode: %s", err)
		test.FailNow()
	}
	if dirp.inum != 539 {
		test.Errorf("Inodes did not match, expected %d, got %d", 539, dirp.inum)
	}
	// Advance to /root/.ssh/known_hosts
	if rip, err = advance(proc, dirp, "known_hosts"); rip == nil {
		test.Logf("Failed to get /root/.ssh/known_hosts inode: %s", err)
		test.FailNow()
	}
	if rip.inum != 540 {
		test.Errorf("Inodes did not match, expected %d, got %d", 540, dirp.inum)
	}
	// Verify the size of the file
	if rip.Size() != 395 {
		test.Errorf("Size of file does not match, expected %d, got %d", 395, rip.Size())
	}

	// Now test the bad or corner cases to ensure the function behaves in a
	// predictable way

	// Look up an entry that doesn't exist
	if dirp, err = advance(proc, proc.rootdir, "monkeybutt"); dirp != nil {
		test.Errorf("Failed when looking up a missing entry, inode not nil: %s", err)
	}

	// Look up an entry on a nil inode
	if dirp, err = advance(proc, nil, "monkeybutt"); dirp != nil {
		test.Errorf("Failed when looking up in a nil inode, inode not nil: %s", err)
	}

	// Look up an entry in a non-directory inode
	if dirp, err = advance(proc, rip, "monkeybutt"); dirp != nil {
		test.Errorf("Failed when looking up on a non-directory inode, inode not nil: %s", err)
	}

	// Look up an empty path
	if dirp, err = advance(proc, proc.rootdir, ""); dirp != proc.rootdir {
		test.Errorf("Failed when looking up with an empty path, inode not the same: %s", err)
		test.Logf("Got %q, expected %q", proc.rootdir, dirp)
	}

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}
