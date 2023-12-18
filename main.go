package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	yamlFile = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${spec.name}-deployment
spec:
  replicas: ${spec.replicas}
  selector:
    matchLabels:
      app: ${resource-1.name}-app
  containers:
  - name: ${resource-1.containerName}`
)

type YamlParser struct {
	tmp string
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// Creates a file in tmp and calls yq to parse the yaml file
// with the yq args
func (y *YamlParser) Parse(raw []byte, yqPath string) (string, error) {
	// create tmp dir
	tempFileName := RandStringBytes(10) + ".yaml"
	fmt.Println(tempFileName)
	// write the raw into tmp file
	err := os.WriteFile(filepath.Join(y.tmp, tempFileName), raw, 0644)
	if err != nil {
		return "", err
	}
	fmt.Println(filepath.Join(y.tmp, tempFileName))
	defer os.Remove(filepath.Join(y.tmp, tempFileName))
	// call yq
	cmd := exec.Command("yq", yqPath, filepath.Join(y.tmp, tempFileName))
	fmt.Println("yq", yqPath, filepath.Join(y.tmp, tempFileName))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func main() {
	y := &YamlParser{tmp: "."}
	a, err := y.Parse([]byte(yamlFile), ".spec.containers")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(a)
}
