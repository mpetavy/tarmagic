package main

import (
	"fmt"
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

func Error(err error) {
	if *debug {
		if err != nil {
			log.Printf("ERROR %s", err.Error())
		}
	}
}

func CreateTempFile() (file *os.File, err error) {
	file, err = os.CreateTemp("", "tarmagic-*")
	if err != nil {
		return nil, err
	}
	defer Error(file.Close())

	Debug(fmt.Sprintf("CreateTempFile : %s", file.Name()))

	return file, err
}

func IsWindows() bool {
	result := runtime.GOOS == "windows"

	return result
}

func CleanPath(path string) string {
	if IsWindows() {
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
