package com.amitia.amitia_app.virtualdisplay.host

import android.os.Looper

object VirtualDisplayHostMain {
    @JvmStatic
    fun main(args: Array<String>) {
        if (args.size < 2) {
            throw IllegalArgumentException("host package and token are required")
        }
        val hostPackage = args[0]
        val token = args[1]
        val context = VirtualDisplayHostEnvironment.context()
        VirtualDisplayHostService(context, hostPackage, token)
        Looper.loop()
    }
}
