package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec

internal class DefaultProotCommandBuilder : ProotCommandBuilder {
    override fun build(spec: ProotLaunchSpec): ProotCommand {
        val args = ArrayList<String>(32)
        if (spec.killOnExit) args.add("--kill-on-exit")
        if (spec.fakeRoot) args.add("-0")
        args.add("-r"); args.add(spec.rootfsPath)
        args.add("-w"); args.add(spec.workingDirectory)
        for (mount in spec.bindMounts) { args.add("-b"); args.add("${mount.host}:${mount.guest}") }
        args.add("--")
        args.addAll(spec.command)
        return ProotCommand(spec.binaryPath, args.toList(), spec.environment.toMap())
    }
}