// Package vendor provides deno vendor integration for tntc deploy.
// It pre-fetches all remote imports (jsr:, https://) from workflow node files
// into a local vendor/ directory so pods have zero outbound network dependency
// at import-resolution time.
//
// The vendor directory is archived as a gzipped tarball (vendor.tar.gz) and
// stored in the workflow ConfigMap's binaryData. At pod startup, the engine's
// start.sh script extracts it to /tmp/vendor/ and passes --import-map to Deno.
package denovendor

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	VendorDir     = "vendor"
	TarballName   = "vendor.tar.gz"
	ImportMapPath = "vendor/import_map.json"
)

// VendorResult holds the outcome of a vendor run.
type VendorResult struct {
	// TarballPath is the absolute path to the created vendor.tar.gz.
	TarballPath string
	// TarballBytes is the raw content of vendor.tar.gz.
	TarballBytes []byte
	// Skipped is true when vendoring was skipped.
	Skipped bool
	// SkipReason explains why (only set when Skipped is true).
	SkipReason string
}

// Run runs `deno vendor` in workflowDir, then archives the result as vendor.tar.gz.
// If deno is not on PATH, returns Skipped=true (warning only, not a hard error).
func Run(workflowDir string, w io.Writer) (*VendorResult, error) {
	if w == nil {
		w = io.Discard
	}

	denoPath, err := exec.LookPath("deno")
	if err != nil {
		return &VendorResult{
			Skipped:    true,
			SkipReason: "deno not found on PATH — remote imports will be resolved at pod startup (will fail under NetworkPolicy)",
		}, nil
	}

	nodesDir := filepath.Join(workflowDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &VendorResult{Skipped: true, SkipReason: "nodes/ directory not found"}, nil
		}
		return nil, fmt.Errorf("reading nodes directory: %w", err)
	}

	var nodeFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".ts" {
			nodeFiles = append(nodeFiles, filepath.Join("nodes", e.Name()))
		}
	}
	if len(nodeFiles) == 0 {
		return &VendorResult{Skipped: true, SkipReason: "no .ts files in nodes/"}, nil
	}

	vendorOut := filepath.Join(workflowDir, VendorDir)

	fmt.Fprintf(w, "  Vendoring remote imports via deno vendor...\n")

	args := []string{"vendor", "--output", vendorOut}
	args = append(args, nodeFiles...)
	cmd := exec.Command(denoPath, args...)
	cmd.Dir = workflowDir
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("deno vendor failed: %w\n\nEnsure all remote imports are reachable from the deploy machine.", err)
	}

	importMap := filepath.Join(workflowDir, ImportMapPath)
	if _, err := os.Stat(importMap); err != nil {
		return nil, fmt.Errorf("deno vendor completed but import_map.json not found at %s", importMap)
	}

	// Archive vendor/ → vendor.tar.gz
	tarballPath := filepath.Join(workflowDir, TarballName)
	tarBytes, err := createTarGz(vendorOut, tarballPath)
	if err != nil {
		return nil, fmt.Errorf("archiving vendor directory: %w", err)
	}

	fmt.Fprintf(w, "  ✓ Vendor archive: %s (%d KB)\n", tarballPath, len(tarBytes)/1024)

	return &VendorResult{
		TarballPath:  tarballPath,
		TarballBytes: tarBytes,
	}, nil
}

// createTarGz archives srcDir as a gzipped tar, writing to dstPath.
// Returns the raw bytes of the archive.
func createTarGz(srcDir, dstPath string) ([]byte, error) {
	f, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("creating tarball: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(filepath.Dir(srcDir), path)
		if err != nil {
			return err
		}
		// Use forward slashes in tar headers (cross-platform)
		relPath = filepath.ToSlash(relPath)

		hdr := &tar.Header{
			Name:    relPath,
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}
		if info.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Name += "/"
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = info.Size()
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.IsDir() {
			rf, err := os.Open(path)
			if err != nil {
				return err
			}
			defer rf.Close()
			if _, err := io.Copy(tw, rf); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Flush and close before reading
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	return os.ReadFile(dstPath)
}
