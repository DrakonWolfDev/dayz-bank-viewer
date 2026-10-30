// Package builds self-contained Windows and Linux releases. Run from the repo root.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := release(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func release() error {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(raw))
	if version == "" || strings.ContainsAny(version, "/\\ \n\r") {
		return fmt.Errorf("invalid VERSION")
	}
	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}
	var sums strings.Builder
	for _, platform := range []string{"linux", "windows"} {
		folder := "dayz-bank-viewer-" + version + "-" + platform + "-amd64"
		stage := filepath.Join("dist", platform+"-amd64")
		if err := os.MkdirAll(stage, 0755); err != nil {
			return err
		}
		binary := "dayz-bank-viewer"
		if platform == "windows" {
			binary += ".exe"
		}
		flags := "-s -w"
		if platform == "windows" {
			flags += " -H=windowsgui"
		}
		cmd := exec.Command("go", "build", "-buildvcs=false", "-trimpath", "-ldflags="+flags, "-o", filepath.Join(stage, binary), ".")
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GOOS=") && !strings.HasPrefix(v, "GOARCH=") && !strings.HasPrefix(v, "CGO_ENABLED=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "GOOS="+platform, "GOARCH=amd64", "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		for _, notice := range []string{"LICENSE", "PATENTS"} {
			data, err := os.ReadFile(filepath.Join("notices", "GO-"+notice))
			if err != nil {
				return err
			}
			if err = os.WriteFile(filepath.Join(stage, "GO-"+notice), data, 0644); err != nil {
				return err
			}
		}
		files := []string{binary, "README.md", "config.example.json", "VERSION", "docs/TESTING.md", "LICENSING.md", "LICENSE", "web/app.ico", "GO-LICENSE", "GO-PATENTS"}
		for _, name := range files[1:] {
			if strings.HasPrefix(name, "GO-") {
				continue
			}
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(stage, name)), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(stage, name), data, 0644); err != nil {
				return err
			}
		}
		name := folder + ".tar.gz"
		if platform == "windows" {
			name = folder + ".zip"
		}
		out := filepath.Join("dist", name)
		if err := archive(out, stage, folder, files, platform == "windows"); err != nil {
			return err
		}
		f, err := os.Open(out)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, f)
		f.Close()
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", hash.Sum(nil), name)
		fmt.Println("Built", out)
	}
	for _, kind := range []string{"deb", "exe"} {
		name, err := installer(version, kind)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(data), filepath.Base(name))
		fmt.Println("Built", name)
	}
	return os.WriteFile(filepath.Join("dist", "SHA256SUMS"), []byte(sums.String()), 0644)
}
func archive(path, stage, folder string, files []string, windows bool) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if e := f.Close(); err == nil {
			err = e
		}
	}()
	var zw *zip.Writer
	var tw *tar.Writer
	if windows {
		zw = zip.NewWriter(f)
		defer func() {
			if e := zw.Close(); err == nil {
				err = e
			}
		}()
	} else {
		gz := gzip.NewWriter(f)
		defer func() {
			if e := gz.Close(); err == nil {
				err = e
			}
		}()
		tw = tar.NewWriter(gz)
		defer func() {
			if e := tw.Close(); err == nil {
				err = e
			}
		}()
	}
	// Explicit allowlist: corpus and local configuration cannot enter an archive.
	for _, name := range files {
		data, e := os.ReadFile(filepath.Join(stage, name))
		if e != nil {
			return e
		}
		mode := int64(0644)
		if name == "dayz-bank-viewer" {
			mode = 0755
		}
		member := folder + "/" + name
		if windows {
			header := &zip.FileHeader{Name: member, Method: zip.Deflate}
			header.SetMode(os.FileMode(mode))
			w, e := zw.CreateHeader(header)
			if e != nil {
				return e
			}
			if _, e = w.Write(data); e != nil {
				return e
			}
		} else {
			if e := tw.WriteHeader(&tar.Header{Name: member, Mode: mode, Size: int64(len(data))}); e != nil {
				return e
			}
			if _, e := tw.Write(data); e != nil {
				return e
			}
		}
	}
	return nil
}
