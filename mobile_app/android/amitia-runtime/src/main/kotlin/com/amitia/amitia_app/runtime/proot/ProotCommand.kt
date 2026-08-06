package com.amitia.amitia_app.runtime.proot

data class ProotCommand(val binaryPath: String, val arguments: List<String>, val environment: Map<String, String>)