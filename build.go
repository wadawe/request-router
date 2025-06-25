//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	version          string = "v1"
	goarch           string
	goos             string
	race             bool = false
	workingDir       string
	serverBinaryName string = "request-router"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	readVersionFromChangelog()
	log.Printf("Building Version: %s", version)

	// Handle command line arguments
	flag.StringVar(&goarch, "goarch", runtime.GOARCH, "GOARCH")
	flag.StringVar(&goos, "goos", runtime.GOOS, "GOOS")
	flag.BoolVar(&race, "race", race, "Use race detector")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Println("Usage: go run build.go build")
		return
	}

	// Handle commands
	workingDir, _ = os.Getwd()
	for _, cmd := range flag.Args() {
		switch cmd {
		case "build":
			pkg := "./pkg/"
			clean()
			build(pkg, []string{}, []string{})
		case "clean":
			clean()
		default:
			log.Fatalf("Unknown command %q", cmd)
		}
	}
}

// Read the version & release from the CHANGELOG.md file
func readVersionFromChangelog() {
	file, err := os.Open("CHANGELOG.md")
	if err != nil {
		log.Fatalf("failed to open CHANGELOG.md: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Regex matches: "# v1.2.3" or "# v 1.2.3-alpha"
	re := regexp.MustCompile(`(?i)^#+\s*v\s*([0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9]+)?)`)

	// Read the file line by line to find the version
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			version = strings.TrimSpace(matches[1])
			return
		}
	}

	// If we reach here, we didn't find a version
	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading CHANGELOG.md: %v", err)
	}
	log.Fatal("version not found in CHANGELOG.md")
}

// Build the router server binary
func build(pkg string, tags []string, flags []string) {
	binary := "./bin/" + serverBinaryName
	if goos == "windows" {
		binary += ".exe"
	}

	// Remove the lasy binary and md5 file
	rmr(binary, binary+".md5")

	// Build the binary
	args := []string{"build", "-ldflags", generateFlags(flags)}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	if race {
		args = append(args, "-race")
	}
	args = append(args, "-v")         // Verbose output
	args = append(args, "-o", binary) // The binary output
	args = append(args, pkg)          // The package to build
	setBuildEnv()
	runPrint("go", "version") // Print the go version
	runPrint("go", args...)   // Build the binary

	// Create an md5 checksum of the binary, to be included in the archive for
	// automatic upgrades.
	err := md5File(binary)
	if err != nil {
		log.Fatal(err)
	}
}

// Generate the ldflags for the build
func generateFlags(flags []string) string {
	var b bytes.Buffer
	b.WriteString("-w")
	b.WriteString(fmt.Sprintf(" -X main.buildVersion=%s", version))
	b.WriteString(fmt.Sprintf(" -X main.buildCommit=%s", getGitSha()))
	for _, f := range flags {
		b.WriteString(fmt.Sprintf(" %s", f))
	}
	return b.String()
}

// Remove a list of files
func rmr(paths ...string) {
	for _, path := range paths {
		log.Println("rm -r", path)
		os.RemoveAll(path)
	}
}

// Clean the build directory
func clean() {
	rmr(filepath.Join(os.Getenv("GOPATH"), fmt.Sprintf("pkg/%s_%s/github.com/wadawe/request-router", goos, goarch)))
}

// Set the build environment
func setBuildEnv() {
	os.Setenv("GOOS", goos)
	if strings.HasPrefix(goarch, "armv") {
		os.Setenv("GOARCH", "arm")
		os.Setenv("GOARM", goarch[4:])
	} else {
		os.Setenv("GOARCH", goarch)
	}
	if goarch == "386" {
		os.Setenv("GO386", "387")
	}
	log.Println("GOPATH=" + os.Getenv("GOPATH"))
}

// Get the git sha from the current commit
func getGitSha() string {
	v, err := runError("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown-dev"
	}
	return string(v)
}

// Run a command and return the output
func run(cmd string, args ...string) []byte {
	bs, err := runError(cmd, args...)
	if err != nil {
		log.Println(cmd, strings.Join(args, " "))
		log.Println(string(bs))
		log.Fatal(err)
	}
	return bytes.TrimSpace(bs)
}

// Run a command and return the output or an error
func runError(cmd string, args ...string) ([]byte, error) {
	ecmd := exec.Command(cmd, args...)
	bs, err := ecmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return bytes.TrimSpace(bs), nil
}

// Run a command and print the output
func runPrint(cmd string, args ...string) {
	log.Println(cmd, strings.Join(args, " "))
	ecmd := exec.Command(cmd, args...)
	ecmd.Stdout = os.Stdout
	ecmd.Stderr = os.Stderr
	err := ecmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// Create an md5 checksum of a file
func md5File(file string) error {

	// Open the file
	fd, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fd.Close()

	// Create the md5 hash
	h := md5.New()
	_, err = io.Copy(h, fd)
	if err != nil {
		return err
	}

	// Write the md5 checksum to a file
	out, err := os.Create(file + ".md5")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%x", h.Sum(nil))
	if err != nil {
		return err
	}
	return out.Close()

}
