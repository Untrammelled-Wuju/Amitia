package com.amitia.amitia_app.runtime.proot

fun interface ProotCommandBuilder { fun build(binaryPath: String, request: ProotLaunchRequest): ProotCommand }