package main

import "bufio"
import "bytes"
import "flag"
import "fmt"
import "log"
import "math"
import "os"
import "strconv"
import "strings"

import "jnwhiteh/minixfs"

var l_ifmt = []byte("0pcCd?dB-?l?s???")

func ModeString(inode *minixfs.Inode) []byte {
	// Start with a default, which we overwrite
	rwx := []byte("drwxr-x--x")

	// This is a dirty hack we inherit from Minix3 to map and file type
	// into a letter for display in ls -l
	rwx[0] = l_ifmt[((inode.Mode)>>12)&0xF]

	mode := inode.Mode & minixfs.RWX_MODES
	index := 1
	for {
		if mode&minixfs.R_BIT > 0 {
			rwx[index+0] = 'r'
		} else {
			rwx[index+0] = '-'
		}
		if mode&minixfs.W_BIT > 0 {
			rwx[index+1] = 'w'
		} else {
			rwx[index+1] = '-'
		}
		if mode&minixfs.X_BIT > 0 {
			rwx[index+2] = 'x'
		} else {
			rwx[index+2] = '-'
		}
		mode = mode >> 3
		index = index + 3
		if index > 7 {
			break
		}
	}

	canexec := inode.Mode&minixfs.X_BIT > 0

	if inode.Mode&minixfs.I_SET_UID_BIT > 0 && canexec {
		rwx[3] = 's'
	}
	if inode.Mode&minixfs.I_SET_GID_BIT > 0 && canexec {
		rwx[6] = 's'
	}

	// TODO: Handle sticky bit
	return rwx
}

// Fetch the file for a given inode block-by-block, printing the data
func PrintFile(fs *minixfs.FileSystem, inode *minixfs.Inode) {
	blocksize := fs.Block_size
	filesize := uint(inode.Size)
	position := uint(0)

	for position < filesize {
		blocknum := fs.ReadMap(inode, position)
		block, err := fs.GetFullDataBlock(blocknum)
		if err != nil {
			fmt.Printf("Failed to get data block: %d - %s\n", blocknum, err)
			break
		}
		if filesize-position >= blocksize {
			fmt.Printf("%s", block.Data)
		} else {
			fmt.Printf("%s", block.Data[:filesize-position])
		}
		position = position + blocksize
	}
	fmt.Printf("\n")
}

func mkdir(fs *minixfs.FileSystem, tokens []string) os.Error {
	// Since the new directory has to have a pointer to the parent, ensure that
	// we can add another link without overflowing Nlinks.
	if fs.WorkDir.Nlinks >= math.MaxUint16 {
		// We cannot add another link to this inode, so fail.
		return os.NewError("Cannot add an extra link to parent directory")
	}
	return nil
}

func repl(filename string, fs *minixfs.FileSystem) {
	fmt.Println("Welcome to the minixfs explorer!")
	fmt.Printf("Attached to %s\n", filename)
	fmt.Printf("Magic number is 0x%x\n", fs.Magic)
	fmt.Printf("Block size: %d\n", fs.Block_size)
	fmt.Printf("Zone shift: %d\n", fs.Log_zone_size)
	fmt.Println("Enter '?' for a list of commands.")

	pwd := []string{}
	buf := bufio.NewReader(os.Stdin)

	for {
	repl:

		// Print the prompt
		fmt.Printf("/%s> ", strings.Join(pwd, "/"))

		// Read another line of input from stdin
		read, err := buf.ReadString('\n')
		if err != nil {
			fmt.Print("\n")
			break
		}

		tokens := strings.Fields(read)
		if len(tokens) == 0 {
			continue
		}

		switch tokens[0] {
		case "?":
			fmt.Println("Commands:")
			fmt.Println("\t?\thelp")
			fmt.Println("\tcat\tshow file contents")
			fmt.Println("\tcd\tchange directory")
			fmt.Println("\tls\tshow directory listing")
			fmt.Println("\tmkdir\tcreate a new directory")
			fmt.Println("\tpwd\tshow current directory")
			fmt.Println("\talloc_bit\tallocate an inode/zone bit")
			fmt.Println("\tfree_bit\tfree an inode/zone bit")
		case "alloc_bit":
			usage := false
			which := uint(0)
			if len(tokens) != 2 {
				usage = true
			} else {
				if tokens[1] == "imap" {
					which = minixfs.IMAP
				} else if tokens[1] == "zmap" {
					which = minixfs.ZMAP
				} else {
					usage = true
				}
			}

			if usage {
				fmt.Println("Usage: alloc_bit [zone|imap]")
				continue
			}

			b := fs.AllocBit(which, 0)
			fmt.Printf("Allocated %s bit number %d\n", tokens[1], b)
		case "cat":
			blocknum := uint(fs.WorkDir.Zone[0])
			dir_block, err := fs.GetDirectoryBlock(blocknum)
			if err != nil {
				fmt.Printf("Failed getting directory block: %d - %s\n", blocknum, err)
			}

			// Loop and find a file with the given name
			filename := tokens[1]
			fileinum := uint(0)

			for _, dirent := range dir_block.Data {
				if dirent.Inum > 0 {
					strend := bytes.IndexByte(dirent.Name[:], 0)
					if strend == -1 {
						strend = len(dirent.Name) - 1
					}
					ename := string(dirent.Name[:strend])
					if ename == filename {
						fileinode, err := fs.GetInode(uint(dirent.Inum))
						if err != nil {
							fmt.Printf("Failed getting inode: %d\n", dirent.Inum)
							break
						}
						if fileinode.IsRegular() {
							fileinum = uint(dirent.Inum)
							fmt.Printf("Found file %s at inode %d\n", filename, fileinum)
							fmt.Printf("Contents:\n")
							PrintFile(fs, fileinode)
							goto repl
						}
					}
				}
			}

			fmt.Printf("Could not find a file named '%s'\n", filename)
		case "cd":
			if len(tokens) < 2 {
				fmt.Println("Usage: cd dirname")
				continue
			}

			blocknum := uint(fs.WorkDir.Zone[0])
			dir_block, err := fs.GetDirectoryBlock(blocknum)
			if err != nil {
				fmt.Printf("Failed getting directory block: %d - %s\n", blocknum, err)
			}

			// Search through the directory entries and find one that
			// matches dirname

			dirname := tokens[1]
			dirinum := uint(0)

			for _, dirent := range dir_block.Data {
				if dirent.Inum > 0 {
					strend := bytes.IndexByte(dirent.Name[:], 0)
					if strend == -1 {
						strend = len(dirent.Name) - 1
					}
					ename := string(dirent.Name[:strend])
					if ename == dirname {
						dirinode, err := fs.GetInode(uint(dirent.Inum))
						if err != nil {
							fmt.Printf("Failed getting inode: %d\n", dirent.Inum)
							break
						}
						if dirinode.IsDirectory() {
							dirinum = uint(dirent.Inum)
							fmt.Printf("Found directory %s at inode %d\n", dirname, dirinum)
							break
						}
					}
				}
			}

			if dirinum == 0 {
				fmt.Printf("Did not find a directory matching '%s'\n", dirname)
			} else if dirinum == fs.WorkDir.Inum() {
				// This would change us to the same directory, do nothing
				continue
			} else {
				newinode, err := fs.GetInode(dirinum)
				if err != nil {
					fmt.Printf("Failed to load inode %d: %s\n", dirinum, err)
					continue
				}

				if dirname == ".." {
					pwd = pwd[:len(pwd)-1]
				} else {
					pwd = append(pwd, tokens[1])
				}

				// Change the fs work directory to the current directory
				fs.WorkDir = newinode
				continue
			}
		case "free_bit":
			usage := false
			which := uint(0)
			bit := uint(0)

			if len(tokens) != 3 {
				usage = true
			} else {
				if tokens[1] == "imap" {
					which = minixfs.IMAP
				} else if tokens[1] == "zmap" {
					which = minixfs.ZMAP
				} else {
					usage = true
				}

				var err os.Error
				bit, err = strconv.Atoui(tokens[2])
				if err != nil {
					usage = true
				}
			}

			if usage {
				fmt.Println("Usage: free_bit <zone|imap> <bitnum>")
				continue
			}

			fs.FreeBit(which, bit)
			fmt.Printf("Freed %s bit number %d\n", tokens[1], bit)
		case "ls":
			blocknum := uint(fs.WorkDir.Zone[0])
			dir_block, err := fs.GetDirectoryBlock(blocknum)
			if err != nil {
				fmt.Printf("Failed getting directory block: %d - %s\n", blocknum, err)
			}

			for _, dirent := range dir_block.Data {
				if dirent.Inum > 0 {
					dirinode, err := fs.GetInode(uint(dirent.Inum))
					if err != nil {
						fmt.Printf("Failed getting inode: %d\n", dirent.Inum)
						break
					}
					mode := ModeString(dirinode)
					fmt.Printf("%s\t%d\t%s\n", mode, dirinode.Nlinks, dirent.Name)
				}
			}
		case "mkdir":
			mkdir(fs, tokens)
		case "pwd":
			fmt.Printf("Current directory is /%s\n", strings.Join(pwd, "/"))
		default:
			fmt.Printf("%s is not a valid command\n", tokens[0])
		}
	}
}

func main() {
	// Set-up and parse flags
	var filename string
	flag.StringVar(&filename, "file", "hello.img", "The filesystem image to explore")

	// Parse the flags from the commandline
	flag.Parse()

	fs, err := minixfs.OpenFileSystemFile(filename)
	if err != nil {
		log.Fatalf("Error opening file system: %s", err)
	}

	repl(filename, fs)
}
