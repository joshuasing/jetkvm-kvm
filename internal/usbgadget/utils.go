package usbgadget

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Helper function to get absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func joinPath(basePath string, paths []string) string {
	pathArr := append([]string{basePath}, paths...)
	return filepath.Join(pathArr...)
}

func ensureSymlink(linkPath string, target string) error {
	if _, err := os.Lstat(linkPath); err == nil {
		currentTarget, err := os.Readlink(linkPath)
		if err != nil || currentTarget != target {
			err = os.Remove(linkPath)
			if err != nil {
				return fmt.Errorf("failed to remove existing symlink %s: %w", linkPath, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if symlink exists: %w", err)
	}

	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink from %s to %s: %w", linkPath, target, err)
	}

	return nil
}

func (u *UsbGadget) writeIfDifferent(filePath string, content []byte, permMode os.FileMode) error {
	if _, err := os.Stat(filePath); err == nil {
		oldContent, err := os.ReadFile(filePath)
		if err == nil {
			if bytes.Equal(oldContent, content) {
				u.log.Tracef("skipping writing to %s as it already has the correct content", filePath)
				return nil
			}

			if len(oldContent) == len(content)+1 &&
				bytes.Equal(oldContent[:len(content)], content) &&
				oldContent[len(content)] == 10 {
				u.log.Tracef("skipping writing to %s as it already has the correct content", filePath)
				return nil
			}

			u.log.Tracef("writing to %s as it has different content old%v new%v", filePath, oldContent, content)
		}
	}
	return os.WriteFile(filePath, content, permMode)
}
