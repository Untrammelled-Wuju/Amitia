package com.amitia.amitia_app.runtime.proot

class ProotBindMount private constructor(val host: String, val guest: String) {
    companion object {
        fun create(host: String, guest: String): ProotBindMount {
            require(host.isNotEmpty() && guest.isNotEmpty()) { "empty path" }
            require(host.startsWith("/") && guest.startsWith("/")) { "must be absolute" }
            require(guest != "/") { "guest cannot be root" }
            require(!host.contains("\u0000") && !guest.contains("\u0000")) { "nul byte in path" }
            require(!host.contains("..")) { "host no traversal" }
            require(!guest.contains("..")) { "guest no traversal" }
            require(!guest.contains(":") && !host.contains(":")) { "no colon" }
            return ProotBindMount(host, guest)
        }
    }
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is ProotBindMount) return false
        return host == other.host && guest == other.guest
    }
    override fun hashCode(): Int = 31 * host.hashCode() + guest.hashCode()
    override fun toString(): String = "$host:$guest"
}