package graph

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/symphony/internal/errors"
)

var (
	referencesRegex = regexp.MustCompile(`\$\{.*\}`)
)

type ReferenceType string

const (
	// ReferenceTypeSpec indicates that the reference is targetting a
	// spec field of the AbstractionClaim resource.
	ReferenceTypeSpec ReferenceType = "spec"
	// ReferenceTypeStatus indicates that the reference is targetting a
	// status field of the AbstractionClaim resource.
	// This is not possible...
	ReferenceTypeStatus ReferenceType = "status"
	// ReferenceTypeAnnotation indicates that the reference is targetting an
	// annotation field of the AbstractionClaim resource.
	ReferenceTypeAnnotation ReferenceType = "annotation"
	// ReferenceTypeMetadata indicates that the reference is targetting a
	// metadata field of the AbstractionClaim resource.
	ReferenceTypeMetadata ReferenceType = "metadata"
	// ReferenceTypeResource indicates that the reference is targetting a
	// another resource that is part of the Abstraction collection.
	ReferenceTypeResource ReferenceType = "resource"
)

func extractReferences(raw []byte) []string {
	matches := referencesRegex.FindAll(raw, -1)
	references := make([]string, len(matches))
	for i, match := range matches {
		references[i] = string(match)
	}
	return references
}

func trimReference(reference string) string {
	reference = strings.TrimLeft(reference, "${")
	reference = strings.TrimRight(reference, "}")
	return reference
}

func buildReference(reference string) (*Reference, error) {
	reference = trimReference(reference)
	parts := strings.Split(reference, ".")
	if len(parts) <= 1 {
		return nil, errors.ErrInvalidReference
	}
	firstPart := parts[0]
	jsonPATH := strings.Join(parts[1:], ".")
	getter, err := resourceGetter(firstPart)
	if err != nil {
		return nil, fmt.Errorf("%v: %v", errors.ErrInvalidReference, err)
	}
	return &Reference{
		Name:              reference,
		Type:              getReferenceType(reference),
		JSONPath:          jsonPATH,
		getTargetResource: getter,
	}, nil
}

type Reference struct {
	Name              string
	Type              ReferenceType
	JSONPath          string
	getTargetResource func([]*Resource) (*Resource, bool)
}

func getReferenceType(reference string) ReferenceType {
	if strings.HasPrefix(reference, "spec.") {
		return ReferenceTypeSpec
	}
	if strings.HasPrefix(reference, "status.") {
		return ReferenceTypeStatus
	}
	if strings.HasPrefix(reference, "metadata.") {
		return ReferenceTypeMetadata
	}
	if strings.HasPrefix(reference, "annotation.") {
		return ReferenceTypeAnnotation
	}
	// Any other reference is a resource reference.
	return ReferenceTypeResource
}

// resourceGetter returns a function that will be used to get the resource.
// a reference has 2 supported formats:
// 1. resouce-name.field-name
// 2. resources[0].field-name
func resourceGetter(referenceFirstPart string) (func([]*Resource) (*Resource, bool), error) {
	if strings.HasPrefix(referenceFirstPart, "resources[") {
		referenceFirstPart = strings.TrimPrefix(referenceFirstPart, "resources[")
		referenceFirstPart = strings.TrimSuffix(referenceFirstPart, "]")
		index, err := strconv.Atoi(referenceFirstPart)
		if err != nil {
			return nil, err
		}

		return func(resources []*Resource) (*Resource, bool) {
			if index >= len(resources) || index < 0 {
				return nil, false
			}
			return resources[index], true
		}, nil
	}
	return func(resources []*Resource) (*Resource, bool) {
		for _, resource := range resources {
			if resource.Name == referenceFirstPart {
				return resource, true
			}
		}
		return nil, false
	}, nil
}

/*
	 func parseDataCELReal(raw []byte, path string) (string, error) {
		var data map[string]interface{}
		err := json.Unmarshal(raw, &raw)
		if err != nil {
			return "", err
		}
		output, err := cel.Eval(path, data)
		if err != nil {
			return "", err
		}
		return output, nil
	}
*/
func parseDataCELFake(raw []byte, path string) (string, error) {
	yq := &YamlParser{tmp: "/Users/hilalymh/source/github.com/aws/symphony/tmp"}
	return yq.Parse(raw, path)
}

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
	cleanRaw := bytes.ReplaceAll(raw, []byte("---\n"), []byte{})
	yqPath = "." + yqPath

	// create tmp dir
	tempFileName := RandStringBytes(10) + ".yaml"
	fmt.Println(tempFileName)
	// write the raw into tmp file
	err := os.WriteFile(filepath.Join(y.tmp, tempFileName), cleanRaw, 0644)
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
	cleanOut := strings.Trim(string(out), "\n")
	cleanOut = strings.TrimSpace(cleanOut)
	return strings.Trim(cleanOut, "\n"), nil
}
