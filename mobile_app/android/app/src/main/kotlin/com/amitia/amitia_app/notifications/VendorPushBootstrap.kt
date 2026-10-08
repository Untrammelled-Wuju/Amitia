package com.amitia.amitia_app.notifications

import android.content.Context
import android.os.Build
import com.amitia.amitia_app.BuildConfig
import java.lang.reflect.Proxy
import java.util.concurrent.Executors

object VendorPushBootstrap {
    private const val PREFS = "amitia_vendor_push"
    private const val INVALIDATED = "invalidated_providers"
    private val executor = Executors.newSingleThreadExecutor()

    fun currentTokens(context: Context): Map<String, String> {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val result = linkedMapOf<String, String>()
        for (provider in listOf("mipush", "hms", "honor", "oppo", "vivo")) {
            val token = prefs.getString(provider, null)?.trim().orEmpty()
            if (token.isNotEmpty()) result[provider] = token
        }
        return result
    }

    fun invalidatedProviders(context: Context): List<String> {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        return prefs.getStringSet(INVALIDATED, emptySet())
            .orEmpty()
            .map { it.trim().lowercase() }
            .filter { it in setOf("mipush", "hms", "honor", "oppo", "vivo") }
            .distinct()
            .sorted()
    }

    fun disable(context: Context) {
        val appContext = context.applicationContext
        val tokens = currentTokens(appContext)
        if (tokens.isNotEmpty()) {
            val prefs = appContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            val invalidated = prefs.getStringSet(INVALIDATED, emptySet())
                .orEmpty()
                .toMutableSet()
            invalidated.addAll(tokens.keys)
            val editor = prefs.edit().putStringSet(INVALIDATED, invalidated)
            tokens.keys.forEach { editor.remove(it) }
            editor.apply()
            tokens.keys.forEach(NotificationPlatformPlugin::emitVendorInvalidated)
        }
        executor.execute {
            disableXiaomi(appContext)
            disableHuawei(appContext)
            disableHonor(appContext)
            disableOppo(appContext)
            disableVivo(appContext)
        }
    }

    fun refresh(context: Context) {
        val appContext = context.applicationContext
        executor.execute {
            refreshXiaomi(appContext)
            refreshHuawei(appContext)
            refreshHonor(appContext)
            refreshOppo(appContext)
            refreshVivo(appContext)
        }
    }

    private fun disableXiaomi(context: Context) {
        runCatching {
            val clazz = Class.forName("com.xiaomi.mipush.sdk.MiPushClient")
            clazz.getMethod("unregisterPush", Context::class.java).invoke(null, context)
        }
    }

    private fun disableHuawei(context: Context) {
        runCatching {
            val appId = BuildConfig.AMITIA_HUAWEI_APP_ID.trim()
            if (appId.isEmpty()) return@runCatching
            val clazz = Class.forName("com.huawei.hms.aaid.HmsInstanceId")
            val instance = clazz.getMethod("getInstance", Context::class.java)
                .invoke(null, context)
                ?: return@runCatching
            clazz.getMethod("deleteToken", String::class.java, String::class.java)
                .invoke(instance, appId, "HCM")
        }
    }

    private fun disableHonor(context: Context) {
        runCatching {
            val namespace = when {
                classAvailable("com.hihonor.push.sdk.HonorPushClient") -> "com.hihonor.push.sdk"
                classAvailable("com.honor.push.sdk.HonorPushClient") -> "com.honor.push.sdk"
                else -> return@runCatching
            }
            val clientClass = Class.forName("$namespace.HonorPushClient")
            val client = clientClass.getMethod("getInstance").invoke(null)
                ?: return@runCatching
            clientClass.methods.firstOrNull {
                (it.name == "turnOffPush" || it.name == "disablePush") &&
                    it.parameterTypes.isEmpty()
            }?.invoke(client)
        }
    }

    private fun disableOppo(context: Context) {
        runCatching {
            val managerClass = Class.forName("com.heytap.msp.push.HeytapPushManager")
            val method = managerClass.methods.firstOrNull {
                (it.name.equals("unRegister", ignoreCase = true) ||
                    it.name.equals("unregister", ignoreCase = true)) &&
                    (it.parameterTypes.isEmpty() ||
                        (it.parameterTypes.size == 1 &&
                            Context::class.java.isAssignableFrom(it.parameterTypes[0])))
            } ?: return@runCatching
            if (method.parameterTypes.isEmpty()) {
                method.invoke(null)
            } else {
                method.invoke(null, context)
            }
        }
    }

    private fun disableVivo(context: Context) {
        runCatching {
            val clientClass = Class.forName("com.vivo.push.PushClient")
            val listenerClass = Class.forName("com.vivo.push.IPushActionListener")
            val client = clientClass.getMethod("getInstance", Context::class.java)
                .invoke(null, context)
                ?: return@runCatching
            val listener = Proxy.newProxyInstance(
                listenerClass.classLoader,
                arrayOf(listenerClass),
            ) { _, _, _ -> null }
            clientClass.getMethod("turnOffPush", listenerClass).invoke(client, listener)
        }
    }

    private fun refreshXiaomi(context: Context) {
        try {
            val appId = BuildConfig.AMITIA_XIAOMI_APP_ID.trim()
            val appKey = BuildConfig.AMITIA_XIAOMI_APP_KEY.trim()
            if (appId.isEmpty() || appKey.isEmpty()) return
            val clazz = Class.forName("com.xiaomi.mipush.sdk.MiPushClient")
            clazz.getMethod(
                "registerPush",
                Context::class.java,
                String::class.java,
                String::class.java,
            ).invoke(null, context, appId, appKey)
            val token = clazz.getMethod("getRegId", Context::class.java)
                .invoke(null, context)
                ?.toString()
                ?.trim()
                .orEmpty()
            store(context, "mipush", token)
        } catch (_: Throwable) {
        }
    }

    private fun refreshHuawei(context: Context) {
        try {
            val appId = BuildConfig.AMITIA_HUAWEI_APP_ID.trim()
            if (appId.isEmpty()) return
            val clazz = Class.forName("com.huawei.hms.aaid.HmsInstanceId")
            val instance = clazz.getMethod("getInstance", Context::class.java)
                .invoke(null, context)
            val token = clazz.getMethod(
                "getToken",
                String::class.java,
                String::class.java,
            ).invoke(instance, appId, "HCM")
                ?.toString()
                ?.trim()
                .orEmpty()
            store(context, "hms", token)
        } catch (_: Throwable) {
        }
    }

    private fun refreshHonor(context: Context) {
        try {
            val namespace = when {
                classAvailable("com.hihonor.push.sdk.HonorPushClient") -> "com.hihonor.push.sdk"
                classAvailable("com.honor.push.sdk.HonorPushClient") -> "com.honor.push.sdk"
                else -> return
            }
            val clientClass = Class.forName("$namespace.HonorPushClient")
            val callbackClass = Class.forName("$namespace.HonorPushCallback")
            val client = clientClass.getMethod("getInstance").invoke(null)
            val supported = clientClass.getMethod(
                "checkSupportHonorPush",
                Context::class.java,
            ).invoke(client, context) as? Boolean ?: false
            if (!supported) return
            clientClass.getMethod(
                "init",
                Context::class.java,
                java.lang.Boolean.TYPE,
            ).invoke(client, context, false)
            val callback = Proxy.newProxyInstance(
                callbackClass.classLoader,
                arrayOf(callbackClass),
            ) { _, method, args ->
                if (method.name == "onSuccess") {
                    val token = args?.firstOrNull()?.toString()?.trim().orEmpty()
                    store(context, "honor", token)
                }
                null
            }
            clientClass.getMethod("getPushToken", callbackClass)
                .invoke(client, callback)
        } catch (_: Throwable) {
        }
    }

    private fun refreshOppo(context: Context) {
        try {
            val appKey = BuildConfig.AMITIA_OPPO_APP_KEY.trim()
            val appSecret = BuildConfig.AMITIA_OPPO_APP_SECRET.trim()
            if (appKey.isEmpty() || appSecret.isEmpty()) return
            val managerClass = Class.forName("com.heytap.msp.push.HeytapPushManager")
            val callbackClass = Class.forName(
                "com.heytap.msp.push.callback.ICallBackResultService",
            )
            managerClass.getMethod(
                "init",
                Context::class.java,
                java.lang.Boolean.TYPE,
            ).invoke(null, context, false)
            val supported = managerClass.getMethod("isSupportPush")
                .invoke(null) as? Boolean ?: false
            if (!supported) return
            val callback = Proxy.newProxyInstance(
                callbackClass.classLoader,
                arrayOf(callbackClass),
            ) { _, method, args ->
                if (method.name == "onRegister") {
                    val code = (args?.getOrNull(0) as? Number)?.toInt() ?: -1
                    val token = args?.getOrNull(1)?.toString()?.trim().orEmpty()
                    if (code == 0) {
                        store(context, "oppo", token)
                    }
                }
                null
            }
            managerClass.getMethod(
                "register",
                Context::class.java,
                String::class.java,
                String::class.java,
                callbackClass,
            ).invoke(null, context, appKey, appSecret, callback)
        } catch (_: Throwable) {
        }
    }

    private fun refreshVivo(context: Context) {
        try {
            if (BuildConfig.AMITIA_VIVO_APP_ID.isBlank() ||
                BuildConfig.AMITIA_VIVO_API_KEY.isBlank()
            ) {
                return
            }
            val clientClass = Class.forName("com.vivo.push.PushClient")
            val listenerClass = Class.forName("com.vivo.push.IPushActionListener")
            val client = clientClass.getMethod("getInstance", Context::class.java)
                .invoke(null, context)
                ?: return
            runCatching {
                clientClass.getMethod("initialize").invoke(client)
            }
            val listener = Proxy.newProxyInstance(
                listenerClass.classLoader,
                arrayOf(listenerClass),
            ) { _, method, args ->
                if (method.name == "onStateChanged") {
                    val state = (args?.firstOrNull() as? Number)?.toInt() ?: -1
                    if (state == 0 || state == 1) {
                        refreshVivoRegId(clientClass, client, context)
                    }
                }
                null
            }
            clientClass.getMethod("turnOnPush", listenerClass)
                .invoke(client, listener)
            refreshVivoRegId(clientClass, client, context)
        } catch (_: Throwable) {
        }
    }

    private fun refreshVivoRegId(
        clientClass: Class<*>,
        client: Any,
        context: Context,
    ) {
        try {
            val noArg = clientClass.methods.firstOrNull {
                it.name == "getRegId" && it.parameterTypes.isEmpty()
            }
            val token = noArg?.invoke(client)?.toString()?.trim().orEmpty()
            if (token.isNotEmpty()) {
                store(context, "vivo", token)
                return
            }
            val queryClass = Class.forName(
                "com.vivo.push.listener.IPushQueryActionListener",
            )
            val callback = Proxy.newProxyInstance(
                queryClass.classLoader,
                arrayOf(queryClass),
            ) { _, method, args ->
                if (method.name == "onSuccess") {
                    val regId = args?.firstOrNull()?.toString()?.trim().orEmpty()
                    store(context, "vivo", regId)
                }
                null
            }
            clientClass.getMethod("getRegId", queryClass)
                .invoke(client, callback)
        } catch (_: Throwable) {
        }
    }

    fun acceptToken(context: Context, provider: String, token: String) {
        store(context.applicationContext, provider.trim().lowercase(), token)
    }

    fun nativeDataProviders(): List<String> {
        val result = mutableListOf<String>()
        if (sdkAvailability()["mipush"] == true &&
            classAvailable("com.amitia.amitia_app.notifications.vendor.AmitiaXiaomiPushReceiver")
        ) {
            result += "mipush"
        }
        if (sdkAvailability()["hms"] == true &&
            classAvailable("com.amitia.amitia_app.notifications.vendor.AmitiaHuaweiMessageService")
        ) {
            result += "hms"
        }
        if (sdkAvailability()["honor"] == true &&
            classAvailable("com.amitia.amitia_app.notifications.vendor.AmitiaHonorMessageService")
        ) {
            result += "honor"
        }
        if (BuildConfig.AMITIA_OPPO_NATIVE_DATA_ENABLED &&
            sdkAvailability()["oppo"] == true &&
            classAvailable("com.amitia.amitia_app.notifications.vendor.AmitiaOppoDataMessageService") &&
            classAvailable("com.amitia.amitia_app.notifications.vendor.AmitiaOppoCompatibleDataMessageService")
        ) {
            result += "oppo"
        }
        if (BuildConfig.AMITIA_VIVO_NATIVE_DATA_ENABLED &&
            sdkAvailability()["vivo"] == true &&
            classAvailable("com.amitia.amitia_app.notifications.vendor.AmitiaVivoPushReceiver")
        ) {
            result += "vivo"
        }
        return result
    }

    private fun store(context: Context, provider: String, token: String) {
        val normalized = token.trim()
        if (normalized.isEmpty()) return
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val previous = prefs.getString(provider, null)?.trim()
        if (previous == normalized) return
        val invalidated = prefs.getStringSet(INVALIDATED, emptySet())
            .orEmpty()
            .toMutableSet()
        invalidated.remove(provider)
        prefs.edit()
            .putString(provider, normalized)
            .putStringSet(INVALIDATED, invalidated)
            .apply()
        NotificationPlatformPlugin.emitVendorToken(provider, normalized)
    }

    fun sdkAvailability(): Map<String, Boolean> = linkedMapOf(
        "mipush" to classAvailable("com.xiaomi.mipush.sdk.MiPushClient"),
        "hms" to classAvailable("com.huawei.hms.aaid.HmsInstanceId"),
        "honor" to (
            classAvailable("com.hihonor.push.sdk.HonorPushClient") ||
                classAvailable("com.honor.push.sdk.HonorPushClient")
            ),
        "oppo" to classAvailable("com.heytap.msp.push.HeytapPushManager"),
        "vivo" to classAvailable("com.vivo.push.PushClient"),
    )

    fun manufacturerHint(): String {
        val name = Build.MANUFACTURER.trim().lowercase()
        return when {
            name.contains("xiaomi") || name.contains("redmi") -> "mipush"
            name.contains("huawei") -> "hms"
            name.contains("honor") -> "honor"
            name.contains("oppo") || name.contains("oneplus") || name.contains("realme") -> "oppo"
            name.contains("vivo") || name.contains("iqoo") -> "vivo"
            else -> ""
        }
    }

    private fun classAvailable(name: String): Boolean = try {
        Class.forName(name)
        true
    } catch (_: Throwable) {
        false
    }
}
