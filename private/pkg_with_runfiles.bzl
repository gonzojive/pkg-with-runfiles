def _pkg_with_runfiles_impl(ctx):
    output_file = ctx.actions.declare_file(ctx.label.name + ".tar")
    binary_spec = ctx.actions.declare_file(ctx.label.name + ".packager_BinarySpec.json")
    processed_inputs = _generate_input_spec(ctx)
    ctx.actions.write(binary_spec, processed_inputs.spec_json)

    ctx.actions.run(
        mnemonic = "PackageWithRunfiles",
        executable = ctx.executable._packager,
        arguments = [
            "--alsologtostderr",
            "--output",
            output_file.path,
            "--spec",
            binary_spec.path,
        ],
        inputs = [
            binary_spec,
        ] + processed_inputs.inputs_to_packager,
        outputs = [output_file],
    )

    return [
        DefaultInfo(files = depset([output_file])),
    ]

def _generate_input_spec(ctx):
    """Returns a BinarySpec json string. See the packager go package."""
    target = ctx.attr.binary
    target_info = target[DefaultInfo]
    target_runfiles = target_info.default_runfiles.merge_all([
        extra_data_label[DefaultInfo].default_runfiles
        for extra_data_label in ctx.attr.extra_data
    ])

    inputs_to_packager = (
        target_info.files.to_list() +
        target_runfiles.files.to_list()
    )

    return struct(
        inputs_to_packager = inputs_to_packager,
        spec_json = json.encode_indent(
            {
                "workspace_name": ctx.workspace_name,
                "executable_name_in_archive": ctx.attr.binary_path_in_archive,
                "binary_target_executable_file": _file_to_dict(
                    target_info.files_to_run.executable,
                ),
                "binary_target_outputs": [
                    _file_to_dict(f)
                    # See https://bazel.build/rules/lib/providers/DefaultInfo#files
                    # for the definition DefaultInfo.files.
                    for f in target_info.files.to_list()
                ],
                "binary_runfiles": _runfiles_to_dict(target_runfiles),
            },
            indent = "  ",
        ),
    )

def _runfiles_to_dict(target_runfiles):
    return {
        "files": [
            _file_to_dict(f)
            for f in target_runfiles.files.to_list()
        ],
        # root_symlinks and symlinks are unused by packager.go but might be
        # useful for debugging if the packager is misbehaving.
        "root_symlinks": [
            {
                "path": symlink.path,
                "target_file": _file_to_dict(symlink.target_file),
            }
            for symlink in target_runfiles.root_symlinks.to_list()
        ],
        "symlinks": [
            {
                "path": symlink.path,
                "target_file": _file_to_dict(symlink.target_file),
            }
            for symlink in target_runfiles.symlinks.to_list()
        ],
    }

def _file_to_dict(file):
    return {
        "path": file.path,
        "is_directory": file.is_directory,
        "is_source": file.is_source,
        "short_path": file.short_path,
        "root": file.root.path,
        "owner": str(file.owner),
    }

pkg_with_runfiles = rule(
    implementation = _pkg_with_runfiles_impl,
    attrs = {
        "binary": attr.label(
            doc = ("An executable target for which the executable itself " +
                   "and any runfiles are being collected."),
            executable = True,
            cfg = "target",
        ),
        "binary_path_in_archive": attr.string(
            doc = "Path to the binary within the generated .tar file.",
            mandatory = True,
        ),
        "extra_data": attr.label_list(
            doc = ("Extra dependencies that should be included as if they " +
                   "were included as data dependencies of the executable."),
            allow_files = True,
        ),
        "_packager": attr.label(
            default = Label("//private:packager"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    },
)
