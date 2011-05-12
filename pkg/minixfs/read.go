package minixfs

import "log"

// Given an inode and a position within the corresponding file, locate the
// block (not zone) number in which that position is to be found and return
func (fs *FileSystem) read_map(inode *Inode, position uint) uint {
	scale := fs.super.Log_zone_size                        // for block-zone conversion
	block_pos := position / fs.super.Block_size            // relative block # in file
	zone := block_pos >> scale                             // position's zone
	boff := block_pos - (zone << scale)                    // relative block in zone
	dzones := uint(V2_NR_DZONES)                           // number of direct zones
	nr_indirects := fs.super.Block_size / V2_ZONE_NUM_SIZE // number of indirect zones

	// Is the position to be found in the inode itself?
	if zone < dzones {
		z := uint(inode.Zone[zone])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		b := (z << scale) + boff
		return b
	}

	// It is not in the inode, so must be single or double indirect
	var z uint
	var excess uint = zone - dzones

	if excess < nr_indirects {
		// 'position' can be located via the single indirect block
		z = uint(inode.Zone[dzones])
	} else {
		// 'position' can be located via the double indirect block
		z = uint(inode.Zone[dzones+1])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		excess = excess - nr_indirects // single indirect doesn't count
		b := z << scale
		bp := fs.get_block(int(b), INDIRECT_BLOCK) // get double indirect block
		zones := bp.block.(IndirectBlock)
		index := excess / nr_indirects
		z = fs.rd_indir(zones, index)    // z= zone for single
		fs.put_block(bp, INDIRECT_BLOCK) // release double indirect block
		excess = excess % nr_indirects   // index into single indirect block
	}

	// 'z' is zone num for single indirect block; 'excess' is index into it
	if z == NO_ZONE {
		return NO_BLOCK
	}

	b := z << scale // b is block number for single indirect
	bp := fs.get_block(int(b), INDIRECT_BLOCK)
	zones := bp.block.(IndirectBlock)
	z = fs.rd_indir(zones, excess)
	fs.put_block(bp, INDIRECT_BLOCK)
	if z == NO_ZONE {
		return NO_BLOCK
	}
	b = (z << scale) + boff
	return b
}

// Given a pointer to an indirect block, read one entry.
func (fs *FileSystem) rd_indir(bp IndirectBlock, index uint) uint {
	zone := uint(bp[index])
	if zone != NO_ZONE && (zone < fs.super.Firstdatazone_old || zone >= fs.super.Zones) {
		log.Printf("Illegal zone number %ld in indirect block, index %d\n", zone, index)
		log.Printf("Firstdatazone_old: %d", fs.super.Firstdatazone_old)
		log.Printf("Nzones: %d", fs.super.Nzones)
		panic("check file system")
	}
	return zone
}
