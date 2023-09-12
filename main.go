package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// tarmagic -f s:\Temp\Marcel\jdk-hotspot-11.0.4_11-mac.tar.gz -d d:\temp\jdk-hotspot-11.0.4_11-mac.tar.gz -o jdk-11.0.4+11

var (
	filename    *string
	destination *string
	offset      *string
	usage       *bool
	debug       *bool
)

func init() {
	filename = flag.String("f", "", "filename TAR/TAR.GZ file to read")
	destination = flag.String("d", "", "directory or filename TAR/TAR.GZ file to write")
	offset = flag.String("o", "", "directory offset to start reading from (optional)")
	usage = flag.Bool("?", false, "show usage (optional)")
	debug = flag.Bool("debug", false, "show debug information (optional)")
}

func gunzip(filename string) (string, error) {
	fmt.Printf("Gunzip of %s ... \n", filename)
	defer func() {
		fmt.Printf("Gunzip of %s done\n", filename)
	}()

	tempFile, err := CreateTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		Error(tempFile.Close())
	}()

	fileTAR, err := os.Create(tempFile.Name())
	if err != nil {
		return "", err
	}
	defer func() {
		Error(fileTAR.Close())
	}()

	fileGZ, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		Error(fileGZ.Close())
	}()

	zr, err := gzip.NewReader(fileGZ)
	if err != nil {
		return "", err
	}

	zr.Multistream(false)

	_, err = io.Copy(fileTAR, zr)
	if err != nil {
		return "", err
	}

	return fileTAR.Name(), nil
}

func gzipp(source string, dest string) error {
	fmt.Printf("Gzip of %s ... \n", dest)
	defer func() {
		fmt.Printf("Gzip of %s done\n", dest)
	}()

	reader, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		Error(reader.Close())
	}()

	writer, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		Error(writer.Close())
	}()

	archiver := gzip.NewWriter(writer)
	defer func() {
		Error(archiver.Close())
	}()

	archiver.Name = filepath.Base(dest)

	_, err = io.Copy(archiver, reader)
	if err != nil {
		return err
	}

	return nil
}

func run() error {
	var err error
	var tarfile *os.File
	var tarball *tar.Writer
	var tarfilename string

	b, err := FileExists(*filename)
	if err != nil {
		return err
	}

	if !b {
		return fmt.Errorf("file not found: %s", *filename)
	}

	if *offset != "" && !strings.HasSuffix(CleanPath(*offset), string(filepath.Separator)) {
		*offset = *offset + string(filepath.Separator)
	}

	if strings.HasSuffix(*destination, ".gz") || strings.HasSuffix(*destination, ".tar") {
		tarfilename = filepath.Join(*destination + ".tar")

		tarfile, err = os.Create(*destination + ".tar")
		if err != nil {
			return err
		}

		tarball = tar.NewWriter(tarfile)
	} else {
		b, err := FileExists(*destination)
		if err != nil {
			return err
		}

		if !b {
			return fmt.Errorf("destination does not exist: %s", *destination)
		}

		b, err = IsDirectory(*destination)
		if err != nil {
			return err
		}

		if !b {
			return fmt.Errorf("destination is not a directory: %s", *destination)
		}
	}

	listSymlinks := make([]string, 0)
	mapSymlinks := make(map[string]string)

	if strings.HasSuffix(*filename, ".gz") {
		var err error

		*filename, err = gunzip(*filename)
		if err != nil {
			return err
		}
	}

	f, err := os.Open(*filename)
	if err != nil {
		return err
	}

	defer func() {
		Error(f.Close())
	}()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		if *offset == "" || (strings.HasPrefix(CleanPath(hdr.Name), CleanPath(*offset)) && len(hdr.Name) > len(*offset)) {
			if tarball != nil {
				header := hdr

				header.Name = hdr.Name[len(*offset):]

				if err := tarball.WriteHeader(header); err != nil {
					return err
				}

				if hdr.FileInfo().IsDir() {
					continue
				}

				if header.Linkname != "" {
					fmt.Printf("Copy entry %s [%s]\n", hdr.Name, hdr.Linkname)
				} else {
					fmt.Printf("Copy entry %s\n", hdr.Name)
				}

				_, err = io.Copy(tarball, tr)
				if err != nil {
					return err
				}

				continue
			}

			dn := filepath.Join(*destination, hdr.Name[len(*offset):])

			dn = CleanPath(dn)
			dir := filepath.Dir(dn)

			if hdr.FileInfo().IsDir() {
				dir = dn
			}

			b, err := FileExists(dir)

			if !b {
				fmt.Printf("Create directory of %s [%s]\n", dn, hdr.Name)

				err = os.MkdirAll(dir, os.ModePerm)
				if err != nil {
					return err
				}
			}

			if hdr.FileInfo().IsDir() {
				continue
			}

			if hdr.Linkname != "" {
				listSymlinks = append(listSymlinks, dn)
				mapSymlinks[dn] = filepath.Clean(hdr.Linkname)

				continue
			}

			fmt.Printf("Untar of %s [%s]\n", dn, hdr.Name)

			dnf, err := os.Create(dn)
			if err != nil {
				return err
			}

			defer func() {
				Error(dnf.Close())
			}()

			if _, err := io.Copy(dnf, tr); err != nil {
				return err
			}

			Error(dnf.Close())
		}
	}

	if tarfile == nil {
		sort.Strings(listSymlinks)

		for _, dn := range listSymlinks {
			ln := mapSymlinks[dn]

			dir := filepath.Dir(dn)

			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			err = os.Chdir(dir)
			if err != nil {
				return err
			}

			target := filepath.Join(dir, ln)

			fmt.Printf("Create link %s [%s]\n", dn, ln)

			targetExists, err := FileExists(target)
			if err != nil {
				return err
			}

			if !targetExists {
				targetDir := filepath.Dir(target)

				b, err := FileExists(targetDir)

				if !b {
					err = os.MkdirAll(targetDir, os.ModePerm)
					if err != nil {
						return err
					}
				}

				f, err = os.Create(target)
				if err != nil {
					return err
				}
				Error(f.Close())
			}

			targetIsDir, err := IsDirectory(target)
			if err != nil {
				return err
			}

			b, err := FileExists(filepath.Base(dn))
			if err != nil {
				return err
			}

			if b {
				err = os.Remove(filepath.Base(dn))
				if err != nil {
					return err
				}
			}

			if IsWindows() {
				if targetIsDir {
					cmd := exec.Command("cmd.exe", "/c", "mklink", "/d", filepath.Base(dn), ln)

					err = cmd.Run()
				} else {
					cmd := exec.Command("cmd.exe", "/c", "mklink", filepath.Base(dn), ln)

					err = cmd.Run()
				}
			} else {
				err = os.Symlink(ln, filepath.Base(dn))
			}
			if err != nil {
				return err
			}

			if !targetExists {
				err = os.Remove(target)
				if err != nil {
					return err
				}
			}

			err = os.Chdir(wd)
			if err != nil {
				return err
			}
		}

		return nil
	}

	Error(tarball.Close())
	Error(tarfile.Close())

	if strings.HasSuffix(*destination, ".gz") {
		err = gzipp(tarfilename, *destination)

		if err != nil {
			return err
		}

		err = os.Remove(tarfilename)

		if err != nil {
			return err
		}
	} else {
		err = os.Rename(tarfilename, *destination)

		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	flag.Parse()

	fmt.Printf("\nTARMAGIC v.1.2 - tool to work with TAR or TAR.GZ files\n\n")

	if *filename == "" || *destination == "" || *usage {
		flag.Usage()
		os.Exit(0)
	}

	err := run()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
}
