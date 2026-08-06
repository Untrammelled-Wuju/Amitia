package com.amitia.amitia_app.runtime.proot

class ProotArtifact private constructor(val componentId: String, val abi: String, val arch: String, val fileName: String, val version: String, val sha256: String) {
    companion object {
        fun create(version: String, sha256: String): ProotArtifact {
            val sha = sha256.lowercase()
            require(sha.matches(Regex("^[0-9a-f]{64}$"))) { "invalid sha256" }
            return ProotArtifact("runtime.proot", "arm64-v8a", "aarch64", "libamitia_proot.so", version, sha)
        }
    }
}