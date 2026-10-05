package com.amitia.amitia_app.runtime.security

import android.content.Context
import android.net.ConnectivityManager
import java.net.Inet4Address

internal class AndroidLANAddresses(context: Context) {
    private val connectivity = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager

    fun addresses(): String = connectivity.allNetworks.flatMap { network ->
        connectivity.getLinkProperties(network)?.linkAddresses.orEmpty().map { it.address }
    }.filter { it is Inet4Address && it.isSiteLocalAddress && !it.isLoopbackAddress }
        .mapNotNull { it.hostAddress }.distinct().sorted().joinToString(",")
}
