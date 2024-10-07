// Program packager assists a bazel rule that builds a docker image layer with runfile support.
package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/golang/glog"
	"golang.org/x/sync/errgroup"
)

var (
	spec       = flag.String("spec", "", "Path to BinarySpec json.")
	outputPath = flag.String("output", "", "Output path of .tar to produce.")
)

func main() {
	flag.Parse()
	// Create and add some files to the archive.
	if err := run(); err != nil {
		glog.Exitf("error: %v", err)
	}
}

func run() error {
	if *outputPath == "" {
		return fmt.Errorf("must specify valid --output path")
	}
	if *spec == "" {
		return fmt.Errorf("must specify valid --spec path")
	}
	specBytes, err := os.ReadFile(*spec)
	if err != nil {
		return fmt.Errorf("error reading input spec: %w", err)
	}
	parsedSpec := &BinarySpec{}
	if err := json.Unmarshal(specBytes, parsedSpec); err != nil {
		return fmt.Errorf("error parsing spec at %s: %w", *spec, err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	if err := writeTarEntries(parsedSpec, tw); err != nil {
		return fmt.Errorf("error writing tar entries: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("error with Close: %w", err)
	}
	if err := os.WriteFile(*outputPath, buf.Bytes(), 0664); err != nil {
		return fmt.Errorf("I/O error writing output .tar file, but the actual contents were already produced successfully: %w", err)
	}
	return nil
}

func writeTarEntries(parsedSpec *BinarySpec, tw *tar.Writer) error {
	var entries []tarEntry
	lock := sync.Mutex{}
	push := func(entry tarEntry) {
		lock.Lock()
		defer lock.Unlock()
		entries = append(entries, entry)
	}

	var allFiles []*File
	allFiles = append(allFiles, parsedSpec.BinaryRunfiles.Files...)
	if parsedSpec.RepoMappingManifest != nil {
		allFiles = append(allFiles, parsedSpec.RepoMappingManifest)
	}

	eg := errgroup.Group{}
	for _, runfile := range allFiles {
		runfile := runfile
		eg.Go(func() error {
			contents, err := os.ReadFile(runfile.Path)
			if err != nil {
				return fmt.Errorf("error reading %q (short_path = %q): %w", runfile.Path, runfile.ShortPath, err)
			}
			fileInfo, err := os.Stat(runfile.Path)
			if err != nil {
				return fmt.Errorf("error calling os.Stat on %q (short_path = %q): %w", runfile.Path, runfile.ShortPath, err)
			}

			push(tarEntry{
				header: &tar.Header{
					Name: nameInOutputArchive(runfile, parsedSpec.WorkspaceName, parsedSpec.BinaryTargetExecutableFile, parsedSpec.RepoMappingManifest, parsedSpec.ExecutableNameInArchive),
					Mode: int64(fileInfo.Mode().Perm()),
					Size: int64(len(contents)),
				},
				contents: contents,
			})
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error generating tar metadata: %w", err)
	}
	// Make output deterministic by sorting filenames.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].header.Name < entries[j].header.Name
	})

	for _, entry := range entries {
		if err := tw.WriteHeader(entry.header); err != nil {
			return fmt.Errorf("error with WriteHeader: %w", err)
		}
		if _, err := tw.Write(entry.contents); err != nil {
			return fmt.Errorf("error with Write: %w", err)
		}
	}
	return nil
}

func nameInOutputArchive(runfile *File, workspaceName string, executable, repoMappingManifest *File, executableNameInArchive string) string {
	// The layout here was inferred from
	// https://docs.google.com/document/d/1skNx5o-8k5-YXUAyEETvr39eKoh9fecJbGUquPh5iy8/edit
	// and from looking at example outputs of executables.
	if runfile.Path == executable.Path {
		return executableNameInArchive
	}
	runfilesRoot := executableNameInArchive + ".runfiles"
	if repoMappingManifest != nil && runfile.Path == repoMappingManifest.Path {
		// The _repo_mapping should appear within the runfiles directory.
		// https://github.com/bazelbuild/proposals/blob/main/designs/2022-07-21-locating-runfiles-with-bzlmod.md#1-emit-a-repository-mapping-manifest-for-each-executable-target
		return path.Join(runfilesRoot, "_repo_mapping")
	}

	// Data dependencies in repositories other than the root repo have prefix "../".
	withoutPrefix := strings.TrimPrefix(runfile.ShortPath, "../")
	if runfile.ShortPath != withoutPrefix {
		return path.Join(runfilesRoot, withoutPrefix)
	} else {
		return path.Join(runfilesRoot, workspaceName, runfile.ShortPath)
	}
}

type tarEntry struct {
	header   *tar.Header
	contents []byte
}

// BinarySpec describes a set of runfiles and a target execution.
type BinarySpec struct {
	// WorkspaceName is the name of the workspace taken from the calling
	// ctx.workspace_name.
	WorkspaceName string `json:"workspace_name"`

	// BinaryTargetExecutableFile is the bazel File object corresponding to
	// the "binary" attribute in the pkg_with_runfiles rule.
	//
	// Runfiles will be placed relative to the location of this file in the
	// generated tar.
	BinaryTargetExecutableFile *File `json:"binary_target_executable_file"`

	// BinaryRunfiles is the set of runfile dependencies of the binary.
	BinaryRunfiles *Runfiles `json:"binary_runfiles"`

	// BinaryTargetOutputs contains information about the executable target.
	//
	// It is often a single file, but it can be multiple, like in the case
	// of java_binary.
	BinaryTargetOutputs []*File `json:"binary_target_outputs"`

	// The name of the file in the output.
	ExecutableNameInArchive string `json:"executable_name_in_archive"`

	// The reppository mapping file used to "translate an apparent repository
	// name to a canonical repository name" according to the [proposal]
	// that added this functionality to Bazel.
	//
	// This file may not be present when bzlmod is not enabled.
	//
	// [proposal]: https://github.com/bazelbuild/proposals/blob/main/designs/2022-07-21-locating-runfiles-with-bzlmod.md
	RepoMappingManifest *File `json:"repo_mapping_manifest"`
}

// Runfiles contains information about a bazel runfiles object.
//
// See https://bazel.build/rules/lib/builtins/runfiles.
type Runfiles struct {
	Files []*File `json:"files"`
}

// File contains information about a bazel File object.
//
// See https://bazel.build/rules/lib/builtins/File.
type File struct {
	IsDirectory bool        `json:"is_directory"`
	IsSource    bool        `json:"is_source"`
	Path        string      `json:"path"`
	ShortPath   string      `json:"short_path"`
	Owner       LabelString `json:"label"`
}

// LabelString is a superficial type for https://bazel.build/rules/lib/builtins/Label.html.
type LabelString string
