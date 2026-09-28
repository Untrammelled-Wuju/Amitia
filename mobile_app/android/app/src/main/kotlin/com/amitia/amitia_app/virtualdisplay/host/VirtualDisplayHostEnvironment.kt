package com.amitia.amitia_app.virtualdisplay.host

import android.annotation.SuppressLint
import android.app.Application
import android.app.Instrumentation
import android.content.Context
import android.content.ContextWrapper
import android.content.pm.ApplicationInfo
import android.os.Build
import java.lang.reflect.Constructor
import java.lang.reflect.Field
import java.lang.reflect.Method

@SuppressLint("PrivateApi", "SoonBlockedPrivateApi")
internal object VirtualDisplayHostEnvironment {
    private const val SHELL_PACKAGE = "com.android.shell"

    @Volatile
    private var context: Context? = null

    fun context(): Context {
        context?.let { return it }
        synchronized(this) {
            context?.let { return it }
            LooperGuard.ensureMainLooper()
            val activityThreadClass = Class.forName("android.app.ActivityThread")
            val activityThread = activityThread(activityThreadClass)
            setStaticField(activityThreadClass, "sCurrentActivityThread", activityThread)
            setBooleanField(activityThreadClass, activityThread, "mSystemThread", true)
            fillBoundApplication(activityThreadClass, activityThread)
            val systemContext = systemContext(activityThreadClass, activityThread)
                ?: throw IllegalStateException("unable to create Android system context")
            val shell = ShellContext(systemContext)
            fillApplication(activityThreadClass, activityThread, shell)
            fillInstrumentation(activityThreadClass, activityThread)
            fillConfigurationController(activityThreadClass, activityThread)
            context = shell
            return shell
        }
    }

    private fun activityThread(activityThreadClass: Class<*>): Any {
        try {
            val constructor = activityThreadClass.getDeclaredConstructor()
            constructor.isAccessible = true
            return constructor.newInstance()
        } catch (_: Throwable) {
            val current = activityThreadClass.getDeclaredMethod("currentActivityThread")
            current.isAccessible = true
            return current.invoke(null)
                ?: throw IllegalStateException("Android ActivityThread unavailable")
        }
    }

    private fun systemContext(activityThreadClass: Class<*>, activityThread: Any): Context? {
        invokeContext(activityThreadClass, activityThread, "getSystemContext")?.let { return it }
        invokeContext(activityThreadClass, activityThread, "getSystemUiContext")?.let { return it }
        val systemMain = activityThreadClass.getDeclaredMethod("systemMain")
        systemMain.isAccessible = true
        val value = systemMain.invoke(null) ?: return null
        setStaticField(activityThreadClass, "sCurrentActivityThread", value)
        return invokeContext(activityThreadClass, value, "getSystemContext")
            ?: invokeContext(activityThreadClass, value, "getSystemUiContext")
    }

    private fun invokeContext(activityThreadClass: Class<*>, activityThread: Any, method: String): Context? {
        return try {
            val declared = activityThreadClass.getDeclaredMethod(method)
            declared.isAccessible = true
            declared.invoke(activityThread) as? Context
        } catch (_: Throwable) {
            null
        }
    }

    private fun fillBoundApplication(activityThreadClass: Class<*>, activityThread: Any) {
        try {
            val bindDataClass = Class.forName("android.app.ActivityThread\$AppBindData")
            val constructor = bindDataClass.getDeclaredConstructor()
            constructor.isAccessible = true
            val bindData = constructor.newInstance()
            val appInfo = ApplicationInfo()
            appInfo.packageName = SHELL_PACKAGE
            setField(bindDataClass, bindData, "appInfo", appInfo)
            setField(activityThreadClass, activityThread, "mBoundApplication", bindData)
        } catch (_: Throwable) {
        }
    }

    private fun fillApplication(activityThreadClass: Class<*>, activityThread: Any, base: Context) {
        val application = Application()
        setField(ContextWrapper::class.java, application, "mBase", base)
        setField(activityThreadClass, activityThread, "mInitialApplication", application)
    }

    private fun fillInstrumentation(activityThreadClass: Class<*>, activityThread: Any) {
        try {
            val field = activityThreadClass.getDeclaredField("mInstrumentation")
            field.isAccessible = true
            if (field.get(activityThread) == null) {
                field.set(activityThread, Instrumentation())
            }
        } catch (_: Throwable) {
        }
    }

    private fun fillConfigurationController(activityThreadClass: Class<*>, activityThread: Any) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return
        try {
            val controllerClass = Class.forName("android.app.ConfigurationController")
            val internalClass = Class.forName("android.app.ActivityThreadInternal")
            val constructor = controllerClass.getDeclaredConstructor(internalClass)
            constructor.isAccessible = true
            val controller = constructor.newInstance(activityThread)
            setField(activityThreadClass, activityThread, "mConfigurationController", controller)
        } catch (_: Throwable) {
        }
    }

    private fun setStaticField(type: Class<*>, name: String, value: Any?) {
        val field = type.getDeclaredField(name)
        field.isAccessible = true
        field.set(null, value)
    }

    private fun setField(type: Class<*>, target: Any, name: String, value: Any?) {
        val field = type.getDeclaredField(name)
        field.isAccessible = true
        field.set(target, value)
    }

    private fun setBooleanField(type: Class<*>, target: Any, name: String, value: Boolean) {
        val field = type.getDeclaredField(name)
        field.isAccessible = true
        field.setBoolean(target, value)
    }

    private class ShellContext(base: Context) : ContextWrapper(base) {
        override fun getPackageName(): String = SHELL_PACKAGE

        override fun getOpPackageName(): String = SHELL_PACKAGE

        override fun getApplicationContext(): Context = this

        override fun createPackageContext(packageName: String, flags: Int): Context = this
    }

    private object LooperGuard {
        fun ensureMainLooper() {
            try {
                Class.forName("android.os.Looper").getDeclaredMethod("prepareMainLooper").invoke(null)
            } catch (_: Throwable) {
            }
        }
    }
}
