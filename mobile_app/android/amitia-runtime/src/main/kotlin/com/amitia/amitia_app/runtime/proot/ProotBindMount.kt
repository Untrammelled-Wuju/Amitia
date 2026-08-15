package com.amitia.amitia_app.runtime.proot

class ProotBindMount private constructor(
    val host: String,
    val guest: String,
    val readOnly: Boolean,
) {
    companion object {
        private fun isAbsoluteStyle(path: String): Boolean {
            if (path.startsWith("/")) return true
            if (path.length >= 2 && path[1] == ':') return true
            return false
        }

        private fun hasBadColon(path: String): Boolean {
            if (path.length >= 2 && path[1] == ':') {
                return path.indexOf(':', 2) >= 0
            }
            return path.contains(":")
        }

        fun create(host: String, guest: String, readOnly: Boolean = false): ProotBindMount {
            require(host.isNotEmpty() && guest.isNotEmpty()) { "empty path" }
            require(isAbsoluteStyle(host) && isAbsoluteStyle(guest)) { "must be absolute" }
            require(guest != "/") { "guest cannot be root" }
            require(!host.contains("\u0000") && !guest.contains("\u0000")) { "nul byte in path" }
            require(!host.contains("..")) { "host no traversal" }
            require(!guest.contains("..")) { "guest no traversal" }
            require(!hasBadColon(host) && !hasBadColon(guest)) { "no extra colon" }
            return ProotBindMount(host, guest, readOnly)
        }
    }
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is ProotBindMount) return false
        return host == other.host && guest == other.guest && readOnly == other.readOnly
    }
    override fun hashCode(): Int = 31 * (31 * host.hashCode() + guest.hashCode()) + readOnly.hashCode()
    override fun toString(): String = "$host:$guest${if (readOnly) ":ro" else ""}"
}