package install

import (
	"errors"
	"fmt"
)

const ChecksumsAssetName = "checksums.txt"

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

func IsUsageError(err error) bool {
	var usageErr *UsageError
	return errors.As(err, &usageErr)
}

type ReleasePlatform struct {
	OS          string
	Arch        string
	ArchiveName string
	BinaryName  string
}

func ReleasePlatformFor(goos, goarch string) (ReleasePlatform, error) {
	os, err := releaseOS(goos)
	if err != nil {
		return ReleasePlatform{}, err
	}
	arch, err := releaseArch(goarch)
	if err != nil {
		return ReleasePlatform{}, err
	}

	binary := "aidlc"
	archive := fmt.Sprintf("aidlc_%s_%s.tar.gz", os, arch)
	if os == "windows" {
		binary = "aidlc.exe"
		archive = fmt.Sprintf("aidlc_windows_%s.zip", arch)
	}

	return ReleasePlatform{
		OS:          os,
		Arch:        arch,
		ArchiveName: archive,
		BinaryName:  binary,
	}, nil
}

func releaseOS(goos string) (string, error) {
	switch goos {
	case "darwin", "linux", "windows":
		return goos, nil
	default:
		return "", &UsageError{Message: fmt.Sprintf("unsupported OS: %s", goos)}
	}
}

func releaseArch(goarch string) (string, error) {
	switch goarch {
	case "amd64", "x86_64":
		return "x86_64", nil
	case "arm64", "aarch64":
		return "arm64", nil
	default:
		return "", &UsageError{Message: fmt.Sprintf("unsupported architecture: %s", goarch)}
	}
}
