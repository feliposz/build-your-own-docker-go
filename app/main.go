//go:build linux
// +build linux

package main

import (
	"archive/tar"
	"compress/gzip"
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

const IMAGES_DIR = "mydocker-images"

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {

	logger, err := os.OpenFile("mydocker.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal("create log file: ", err)
	}
	log.SetOutput(logger)
	defer logger.Close()

	log.Println("=========================")

	log.Println("processing command line: ", os.Args)

	image := os.Args[2]
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	manifest, _, err := getDockerImage(image)
	if err != nil {
		log.Fatal("getting docker image: ", err)
	}

	log.Println("making temp dir")
	tempDir, err := os.MkdirTemp(os.TempDir(), "mydocker")
	if err != nil {
		log.Fatal("making temp directory: ", err)
	}

	err = unpackLayers(tempDir, manifest.Layers)
	if err != nil {
		log.Fatal("unpacking layers: ", err)
	}

	log.Println("preparing chroot environment")
	commandPath, err := filepath.Abs(command)
	if err != nil {
		log.Fatal("getting path: ", err)
	}
	commandPathTemp := filepath.Join(tempDir, commandPath)

	// If command executable is already present in unpacked image, skip copying

	_, err = os.Stat(commandPathTemp)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal("stat: ", err)
		}

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

	log.Println("removing temp dir")
	err = os.RemoveAll(tempDir)
	if err != nil {
		log.Println("error removing directory: ", err)
	}

	os.Exit(exitCode)
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

// https://distribution.github.io/distribution#token-response-fields

type Auth struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	IssuedAt  string `json:"issued_at"`
}

// https://distribution.github.io/distribution#image-manifest

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

// https://github.com/opencontainers/image-spec/blob/main/schema/config-schema.json

type Config struct {
	Architecture    string          `json:"architecture"`
	Config          ContainerConfig `json:"config"`
	Container       string          `json:"container"`
	ContainerConfig ContainerConfig `json:"container_config"`
	Created         string          `json:"created"`
	Docker_version  string          `json:"docker_version"`
	History         []ConfigHistory `json:"history"`
	Os              string          `json:"os"`
	Rootfs          ConfigRootFS    `json:"rootfs"`
}

type ContainerConfig struct {
	Hostname     string
	Domainname   string
	User         string
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	Tty          bool
	OpenStdin    bool
	StdinOnce    bool
	Env          []string
	Cmd          []string
	Image        string
	Volumes      interface{}
	WorkingDir   string
	Entrypoint   interface{}
	OnBuild      interface{}
	Labels       interface{}
}

type ConfigHistory struct {
	Created    string `json:"created"`
	CreatedBy  string `json:"created_by"`
	EmptyLayer bool   `json:"empty_layer"`
}

type ConfigRootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

func getDockerImage(image string) (manifest Manifest, config Config, err error) {
	parts := strings.Split(image, ":")
	if len(parts) == 1 {
		parts = append(parts, "latest")
	}

	log.Println("get an auth token")

	// https://distribution.github.io/distribution#getting-a-bearer-token
	// https://distribution.github.io/distribution#using-the-signed-token

	authResp, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", parts[0]))
	if err != nil {
		return
	}
	defer authResp.Body.Close()

	authData, err := io.ReadAll(authResp.Body)
	if err != nil {
		return
	}
	var auth Auth
	json.Unmarshal(authData, &auth)

	// https://distribution.github.io/distribution/#pulling-an-image-manifest

	log.Println("get the manifest for ", image)

	manifestReq, err := http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/%s", parts[0], parts[1]), nil)
	manifestReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.Token))
	manifestReq.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	manifestResp, err := http.DefaultClient.Do(manifestReq)
	if err != nil {
		return
	}
	defer manifestResp.Body.Close()
	manifestData, err := io.ReadAll(manifestResp.Body)
	if err != nil {
		return
	}
	json.Unmarshal(manifestData, &manifest)

	err = os.Mkdir(IMAGES_DIR, 0755)
	if err != nil {
		if !os.IsExist(err) {
			return
		}
	}

	// https://distribution.github.io/distribution#pulling-a-layer

	log.Println("downloading config ", manifest.Config.Digest)
	configReq, err := http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/blobs/%s", parts[0], manifest.Config.Digest), nil)
	configReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.Token))
	configReq.Header.Add("Accept", manifest.Config.MediaType)
	configResp, err := http.DefaultClient.Do(configReq)
	if err != nil {
		return
	}
	defer configResp.Body.Close()
	configData, err := io.ReadAll(configResp.Body)
	if err != nil {
		return
	}
	json.Unmarshal(configData, &config)

	for _, layer := range manifest.Layers {
		layerPath := filepath.Join(IMAGES_DIR, strings.Replace(layer.Digest, ":", "_", 1))

		_, err = os.Stat(layerPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return
			}
		} else {
			log.Println("skipping download for layer ", layer.Digest)
			continue
		}

		var file *os.File
		file, err = os.Create(layerPath)
		if err != nil {
			return
		}
		defer file.Close()

		// https://distribution.github.io/distribution#pulling-a-layer

		log.Println("downloading layer ", layer.Digest)
		var layerReq *http.Request
		layerReq, err = http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/blobs/%s", parts[0], layer.Digest), nil)
		layerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.Token))
		layerReq.Header.Add("Accept", layer.MediaType)
		var layerResp *http.Response
		layerResp, err = http.DefaultClient.Do(layerReq)
		if err != nil {
			return
		}
		defer layerResp.Body.Close()
		var layerData []byte
		layerData, err = io.ReadAll(layerResp.Body)
		if err != nil {
			return
		}
		file.Write(layerData)
	}

	return
}

func unpackLayers(targetDir string, layers []ManifestConfig) error {
	for _, layer := range layers {
		layerPath := filepath.Join(IMAGES_DIR, strings.Replace(layer.Digest, ":", "_", 1))

		log.Println("unpacking layer: ", layer.Digest)

		layerFile, err := os.Open(layerPath)
		if err != nil {
			return err
		}
		defer layerFile.Close()

		gzReader, err := gzip.NewReader(layerFile)
		if err != nil {
			return err
		}

		tarReader := tar.NewReader(gzReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			path := filepath.Join(targetDir, header.Name)

			switch header.Typeflag {

			case tar.TypeSymlink:
				log.Println("creating symlink ", path)
				absolutePath := filepath.Join(targetDir, header.Linkname)
				relativePath, err := filepath.Rel(filepath.Dir(path), absolutePath)
				if err != nil {
					return err
				}
				err = os.Symlink(relativePath, path)
				if err != nil {
					return err
				}

			case tar.TypeDir:
				log.Println("unpacking dir ", path)
				err := os.MkdirAll(path, header.FileInfo().Mode())
				if err != nil {
					return err
				}

			case tar.TypeReg:
				log.Println("unpacking regular file ", path)
				file, err := os.Create(path)
				if err != nil {
					return err
				}
				defer file.Close()
				_, err = io.Copy(file, tarReader)
				if err != nil {
					return err
				}

			default:
				fmt.Printf("dir: %#v\n", header)
				panic(fmt.Sprintf("not implemented: %c", header.Typeflag))

			}

			err = os.Chmod(path, header.FileInfo().Mode())
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			}

			// err = os.Chown(path, header.Uid, header.Gid)
			// if err != nil {
			// 	return err
			// }
		}
	}
	return nil
}
