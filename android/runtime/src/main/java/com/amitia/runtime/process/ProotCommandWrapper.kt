package com.amitia.runtime.process

import java.io.File

interface ProotCommandWrapper {

    fun isProotAvailable(): Boolean

    fun wrap(
        command: List<String>,
        env: Map<String, String>,
        workDir: File
    ): List<String>

    fun fallbackReason(): String?
}
