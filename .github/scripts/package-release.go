package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	plugin := flag.String("plugin", "", "plugin name")
	version := flag.String("version", "", "release version")
	goos := flag.String("goos", "", "target GOOS")
	goarch := flag.String("goarch", "", "target GOARCH")
	binary := flag.String("binary", "", "built plugin binary")
	dist := flag.String("dist", "dist", "output directory")
	flag.Parse()

	require("plugin", *plugin)
	require("version", *version)
	require("goos", *goos)
	require("goarch", *goarch)
	require("binary", *binary)

	ext := extension(*goos)
	zipName := fmt.Sprintf("%s_%s_%s_%s.zip", *plugin, strings.TrimPrefix(*version, "v"), *goos, *goarch)
	zipPath := filepath.Join(*dist, zipName)

	if err := os.MkdirAll(*dist, 0o755); err != nil {
		fatal(err)
	}
	if err := createZip(zipPath, *binary, *plugin+"."+ext); err != nil {
		fatal(err)
	}
	sum, err := sha256File(zipPath)
	if err != nil {
		fatal(err)
	}
	sumLine := fmt.Sprintf("%s  %s\n", sum, zipName)
	if err := os.WriteFile(zipPath+".sha256", []byte(sumLine), 0o644); err != nil {
		fatal(err)
	}
	fmt.Print(sumLine)
}

func require(name, value string) {
	if strings.TrimSpace(value) == "" {
		fatal(fmt.Errorf("missing -%s", name))
	}
}

func extension(goos string) string {
	switch goos {
	case "darwin":
		return "dylib"
	case "windows":
		return "dll"
	default:
		return "so"
	}
}

func createZip(zipPath, binaryPath, entryName string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	info, err := os.Stat(binaryPath)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = entryName
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(writer, in)
	return err
}

func sha256File(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, in); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
