package com.amitia.nativeproot

object ProotBridge {

    private var loaded: Boolean = false
    private var loadError: String? = null

    init {
        try {
            System.loadLibrary("proot")
            loaded = true
        } catch (e: UnsatisfiedLinkError) {
            loadError = e.message
        } catch (e: SecurityException) {
            loadError = e.message
        }
    }

    fun isLoaded(): Boolean = loaded

    fun loadError(): String? = loadError

    external fun prootMain(args: Array<String>): Int

    external fun prootVersion(): String

    external fun prootSetTrace(enabled: Boolean)

    external fun prootShutdown(): Int
}
