package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	targets := []struct {
		os   string
		arch []string
	}{
		{"darwin", []string{"386", "amd64"}},
		{"dragonfly", []string{"amd64"}},
		{"freebsd", []string{"386", "amd64", "arm"}},
		{"netbsd", []string{"386", "amd64", "arm"}},
		{"openbsd", []string{"386", "amd64", "arm"}},
		{"plan9", []string{"386", "amd64", "arm"}},
		{"linux", []string{"386", "amd64", "arm", "arm64", "ppc64", "ppc64le", "mips", "mipsle", "mips64", "mips64le", "s390x"}},
	}

	for _, t := range targets {
		for _, arch := range t.arch {
			build := exec.Command("go", "build")
			build.Stderr = os.Stderr
			build.Stdout = os.Stdout
			build.Env = append(os.Environ(), "GOOS="+t.os, "GOARCH="+arch)
			if err := build.Run(); err != nil {
				panic(err)
			}
			zip := exec.Command("zip", "")
			zip.Stderr = os.Stderr
			zip.Stdout = os.Stdout
			zip.Args = []string{"-9", fmt.Sprintf("dumb-mp3-streamer_%s_%s.zip", t.os, arch), "dumb-mp3-streamer", "LICENSE", "README.md"}
			if err := zip.Run(); err != nil {
				panic(err)
			}
			if err := os.Remove("dumb-mp3-streamer"); err != nil {
				panic(err)
			}
		}
	}
}
