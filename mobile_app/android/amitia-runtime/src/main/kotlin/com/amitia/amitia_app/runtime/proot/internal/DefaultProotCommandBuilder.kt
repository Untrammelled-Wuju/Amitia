package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec

internal class DefaultProotCommandBuilder : ProotCommandBuilder {
    override fun build(spec: ProotLaunchSpec): ProotCommand {
        val args = ArrayList<String>(32)
        if (spec.killOnExit) args.add("--kill-on-exit")
        if (spec.fakeRoot) args.add("-0")
        args.add("-r")
        args.add(spec.rootfsPath)
        args.add("-w")
        args.add(spec.workingDirectory)

        for (mount in spec.bindMounts) {
            // PRoot bind syntax is host:guest. Appending Docker-style ":ro"
            // makes the guest path invalid on Android PRoot and can terminate
            // the outer session before the backend health detector starts.
            // The immutable program tree is protected by the installer/version
            // contract rather than by an unsupported bind suffix.
            args.add("-b")
            args.add("${mount.host}:${mount.guest}")
        }

        args.add("--")
        args.addAll(spec.command)
        return ProotCommand(spec.binaryPath, args.toList(), spec.environment.toMap())
    }
}
