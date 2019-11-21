package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	UserHomeDir string
)

func init() {
	usr, err := user.Current()
	if err == nil {
		UserHomeDir = usr.HomeDir
	}
}

func Debug(v ...interface{}) {
	if *debug {
		log.Printf("DEBUG %s", fmt.Sprint(v...))
	}
}

func WarnError(err error) {
	if *debug {
		if err != nil {
			log.Printf("ERROR %s", err.Error())
		}
	}
}

func CreateTempFile() (file *os.File, err error) {
	file, err = ioutil.TempFile("", "tarmagic-*")
	if err != nil {
		return nil, err
	}
	defer WarnError(file.Close())

	Debug(fmt.Sprintf("CreateTempFile : %s", file.Name()))

	return file, err
}

// CreateTempDir creates a temporary file
func CreateTempDir() (string, error) {
	tempdir, err := ioutil.TempDir("", "tarmagic-*")
	if err != nil {
		return "", err
	}

	Debug(fmt.Sprintf("CreateTempDir : %s", tempdir))

	return tempdir, err
}

func IsWindowsOS() bool {
	result := runtime.GOOS == "windows"

	return result
}

func Executable() string {
	path, err := os.Executable()
	if err != nil {
		path = os.Args[0]
	}

	isMain := strings.Index(filepath.Base(path), "main") != -1

	if isMain {
		wd, err := os.Getwd()
		if err == nil {
			path = filepath.Join(wd, filepath.Base(wd)+filepath.Ext(path))
		}
	}

	return path
}

func CleanPath(path string) string {
	if IsWindowsOS() {
		path = strings.Replace(path, "/", string(filepath.Separator), -1)
	} else {
		path = strings.Replace(path, "\\", string(filepath.Separator), -1)
	}

	p := strings.Index(path, "~")

	if p != -1 {
		path = strings.Replace(path, "~", UserHomeDir, -1)
	}

	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err == nil {
			path = filepath.Join(cwd, path)
		}
	}

	return path
}

func FileExists(filename string) (bool, error) {
	var b bool
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		b = false
		err = nil
	} else {
		b = err == nil
	}

	Debug(fmt.Sprintf("FileExists %s: %v", filename, b))

	return b, err
}

func IsDirectory(path string) (bool, error) {
	b, err := FileExists(path)
	if err != nil {
		return false, err
	}

	if b {
		fi, err := os.Stat(path)
		if err != nil {
			return false, err
		}

		return fi.IsDir(), nil
	} else {
		return false, nil
	}
}
