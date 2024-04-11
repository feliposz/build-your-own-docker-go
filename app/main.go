//go:build linux
// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	image := os.Args[2]
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	getDockerImage(image)

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
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWPID | syscall.CLONE_NEWUSER

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

type Auth struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	IssuedAt  string `json:"issued_at"`
}

type Manifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType"`
	Config        ManifestConfig   `json:"config"`
	Layers        []ManifestConfig `json:"layers"`
}

type ManifestConfig struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}

func getDockerImage(image string) error {
	parts := strings.Split(image, ":")
	if len(parts) == 1 {
		parts = append(parts, "latest")
	}

	log.Println("get an auth token")

	authResp, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", parts[0]))
	if err != nil {
		return err
	}
	defer authResp.Body.Close()

	authData, err := io.ReadAll(authResp.Body)
	if err != nil {
		return err
	}
	var auth Auth
	json.Unmarshal(authData, &auth)

	log.Println("get the manifest for ", image)

	manifestReq, err := http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/%s", parts[0], parts[1]), nil)
	manifestReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.Token))
	manifestReq.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	manifestResp, err := http.DefaultClient.Do(manifestReq)
	if err != nil {
		return err
	}
	defer manifestResp.Body.Close()
	manifestData, err := io.ReadAll(manifestResp.Body)
	if err != nil {
		return err
	}
	var manifest Manifest
	json.Unmarshal(manifestData, &manifest)

	fmt.Println(manifest)

	return nil
}
