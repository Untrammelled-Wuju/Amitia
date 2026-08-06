package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest

internal class DefaultProotCommandBuilder : ProotCommandBuilder {
    override fun build(binaryPath: String, request: ProotLaunchRequest): ProotCommand {
        val args = ArrayList<String>(32)
        if (request.killOnExit) args.add("--kill-on-exit")
        if (request.fakeRoot) args.add("-0")
        args.add("-r"); args.add(request.rootfsPath)
        args.add("-w"); args.add(request.workingDirectory)
        for (mount in request.bindMounts) { args.add("-b"); args.add("${mount.host}:${mount.guest}") }
        args.add("--")
        args.addAll(request.command)
        return ProotCommand(binaryPath, args.toList(), request.environmentSource.toMap())
    }
}