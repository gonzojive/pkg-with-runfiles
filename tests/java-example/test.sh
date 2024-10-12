#!/bin/bash

# For why, see
# https://gist.github.com/mohanpedala/1e2ff5661761d3abd0385e8223e16425
set -euxo pipefail

cd repo1
bazel build //com/example:pkg

# Create a temporary directory
temp_dir=$(mktemp -d)

# Ensure cleanup of the temporary directory on exit
trap 'rm -rf "$temp_dir"' EXIT

# Copy the tar file to the temporary directory
cp "bazel-bin/com/example/pkg.tar" "$temp_dir/"

# Extract the tar file
tar -xf "$temp_dir/pkg.tar" -C "$temp_dir"

# Run the program
"$temp_dir/foo/program" --singlejar
