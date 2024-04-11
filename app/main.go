package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {

	logger, err := os.Create("mydocker.log")
	if err != nil {
		log.Fatal("create log file: ", err)
	}
	log.SetOutput(logger)
	defer logger.Close()

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	log.Println("making temp dir")
	tempDir, err := os.MkdirTemp(os.TempDir(), "mydocker")
	if err != nil {
		log.Fatal("making temp directory: ", err)
	}

	log.Println("preparing chroot environment")
	commandPath, err := filepath.Abs(command)
	if err != nil {
		log.Fatal("getting path: ", err)
	}
	commandPathTemp := filepath.Join(tempDir, commandPath)
	err = os.MkdirAll(filepath.Dir(commandPathTemp), 0755)
	if err != nil {
		log.Fatal("making directory: ", err)
	}

	log.Println("copying executable")
	err = copyFile(commandPath, commandPathTemp)
	if err != nil {
		log.Fatal("copying file: ", err)
	}
	err = os.Chmod(commandPathTemp, 0755)
	if err != nil {
		log.Fatal("chmod file: ", err)
	}

	log.Println("running command with args")
	log.Println("command: ", command)
	log.Println("args: ", args)
	log.Println("chroot: ", tempDir)
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Chroot = tempDir
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWPID

	exitCode := 0
	err = cmd.Run()
	if err != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	//os.RemoveAll(tempDir)

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}
