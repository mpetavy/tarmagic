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
	"time"

	"github.com/mpetavy/common"
)

// tarmagic -f s:\Temp\Marcel\jdk-hotspot-11.0.4_11-mac.tar.gz -d d:\temp\jdk-hotspot-11.0.4_11-mac.tar.gz -o jdk-11.0.4+11

var (
	filename    *string
	destination *string
	offset      *string
)

func init() {
	filename = flag.String("f", "", "filename")
	destination = flag.String("d", "", "destination directory or file")
	offset = flag.String("o", "", "offset")
}

func gunzip(filename string) (string, error) {
	fmt.Printf("Gunzip of %s ... \n", filename)
	defer func() {
		fmt.Printf("Gunzip of %s done\n", filename)
	}()

	tempFile, err := common.CreateTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		common.DebugError(tempFile.Close())
	}()

	fileTAR, err := os.Create(tempFile.Name())
	if err != nil {
		return "", err
	}
	defer func() {
		common.DebugError(fileTAR.Close())
	}()

	fileGZ, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		common.DebugError(fileGZ.Close())
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
		common.DebugFunc(reader.Close())
	}()

	writer, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		common.DebugFunc(writer.Close())
	}()

	archiver := gzip.NewWriter(writer)
	defer func() {
		common.DebugFunc(archiver.Close())
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

	if *offset != "" && !strings.HasSuffix(common.CleanPath(*offset), string(filepath.Separator)) {
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
		b, err := common.FileExists(*destination)
		if err != nil {
			return err
		}

		if !b {
			return fmt.Errorf("destination does not exist: %s", *destination)
		}

		b, err = common.IsDirectory(*destination)
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
		common.DebugError(f.Close())
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

		if *offset == "" || (strings.HasPrefix(common.CleanPath(hdr.Name), common.CleanPath(*offset)) && len(hdr.Name) > len(*offset)) {
			if tarball != nil {
				fmt.Printf("Copy entry %s\n", hdr.Name)

				header, err := tar.FileInfoHeader(hdr.FileInfo(), hdr.Name)
				if err != nil {
					return err
				}

				header.Name = hdr.Name[len(*offset):]

				if err := tarball.WriteHeader(header); err != nil {
					return err
				}

				if hdr.FileInfo().IsDir() {
					continue
				}

				_, err = io.Copy(tarball, tr)
				if err != nil {
					return err
				}

				continue
			}

			dn := filepath.Join(*destination, hdr.Name[len(*offset):])

			dn = common.CleanPath(dn)
			dir := filepath.Dir(dn)

			if hdr.FileInfo().IsDir() {
				dir = dn
			}

			b, err := common.FileExists(dir)

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
				common.DebugError(dnf.Close())
			}()

			if _, err := io.Copy(dnf, tr); err != nil {
				return err
			}

			common.DebugError(dnf.Close())
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

			targetExists, err := common.FileExists(target)
			if err != nil {
				return err
			}

			if !targetExists {
				targetDir := filepath.Dir(target)

				b, err := common.FileExists(targetDir)

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
				common.DebugError(f.Close())
			}

			targetIsDir, err := common.IsDirectory(target)
			if err != nil {
				return err
			}

			b, err := common.FileExists(filepath.Base(dn))
			if err != nil {
				return err
			}

			if b {
				err = os.Remove(filepath.Base(dn))
				if err != nil {
					return err
				}
			}

			if common.IsWindowsOS() {
				if targetIsDir {
					cmd := exec.Command("cmd.exe", "/c", "mklink", "/d", filepath.Base(dn), ln)

					err = cmd.Run()
				} else {
					cmd := exec.Command("cmd.exe", "/c", "mklink", filepath.Base(dn), ln)

					err = cmd.Run()
				}
			} else {
				err = os.Symlink(filepath.Base(dn), ln)
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

	common.DebugFunc(tarball.Close())
	common.DebugFunc(tarfile.Close())

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
	defer common.Cleanup()

	common.New(&common.App{common.Title(), "1.0.0", "2019", common.Title(), "mpetavy", common.APACHE, "https://github.com/golang/mpetavy/golang/" + common.Title(), false, nil, nil, run, time.Duration(0)}, []string{"f", "d"})
	common.Run()
}
