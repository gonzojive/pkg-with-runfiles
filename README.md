# pkg-with-runfiles
A bazel rule for generating tar/zip with an executable and its runfile dependencies

# Usage

## Example

```starlark
# MODULE.bazel
module(
    name = "rooty",
    version = "0.1",
)

bazel_dep(name = "pkg_with_runfiles", version = "0.1")
```

```starlark
# BUILD.bazel

load("@rules_go//go:def.bzl", "go_binary", "go_library")
load("@pkg_with_runfiles//:defs.bzl", "pkg_with_runfiles")


pkg_with_runfiles(
    name = "rooty_packaged",
    binary = ":rooty",
    binary_path_in_archive = "foo/program",
    extra_data = [],
)

go_binary(
    name = "rooty",
    embed = [":rooty_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "rooty_lib",
    srcs = ["main.go"],
    data = [
        "@data1_from_rooty//:message.txt",
        ":unused.txt",
    ],
    importpath = "example.com/rooty",
    visibility = ["//visibility:private"],
    deps = ["@rules_go//go/runfiles:go_default_library"],
)
```

Build the tar, extract it, and run the program inside.

```shell
cd rooty
bazel build //:rooty_packaged

# Create a temporary directory
temp_dir=$(mktemp -d)

# Copy the tar file to the temporary directory
cp "bazel-bin/rooty_packaged.tar" "$temp_dir/"

# Extract the tar file
tar -xf "$temp_dir/rooty_packaged.tar" -C "$temp_dir"

# Run the program
"$temp_dir/foo/program"
```
