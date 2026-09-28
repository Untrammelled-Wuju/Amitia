package com.amitia.amitia_app.virtualdisplay.host

import android.content.Context
import android.os.Bundle
import android.os.IBinder
import android.os.Parcel
import android.os.Parcelable

internal object VirtualDisplayHostContract {
    const val ACTION_READY = "com.amitia.amitia_app.action.VIRTUAL_DISPLAY_HOST_READY"
    const val EXTRA_TOKEN = "token"
    const val EXTRA_CONTAINER = "container"
    const val PREFERENCES_NAME = "amitia_virtual_display_host"
    const val KEY_TOKEN = "host_token"
    const val KEY_PROCESS_ID = "host_process_id"
    const val KEY_AI_ENABLED = "ai_enabled"
    const val HOST_CLASS = "com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayHostMain"
}

class VirtualDisplayHostBinderContainer(
    val binder: IBinder,
) : Parcelable {
    private constructor(parcel: Parcel) : this(
        parcel.readStrongBinder() ?: throw IllegalStateException("virtual display host binder missing"),
    )

    override fun writeToParcel(dest: Parcel, flags: Int) {
        dest.writeStrongBinder(binder)
    }

    override fun describeContents(): Int = 0

    companion object CREATOR : Parcelable.Creator<VirtualDisplayHostBinderContainer> {
        override fun createFromParcel(parcel: Parcel): VirtualDisplayHostBinderContainer =
            VirtualDisplayHostBinderContainer(parcel)

        override fun newArray(size: Int): Array<VirtualDisplayHostBinderContainer?> =
            arrayOfNulls(size)
    }
}

internal object VirtualDisplayHostBinderRegistry {
    @Volatile
    private var service: IVirtualDisplayHost? = null

    fun accept(context: Context, binder: IBinder, token: String): Boolean {
        val expected = context.getSharedPreferences(
            VirtualDisplayHostContract.PREFERENCES_NAME,
            Context.MODE_PRIVATE,
        ).getString(VirtualDisplayHostContract.KEY_TOKEN, null)
        if (expected.isNullOrBlank() || token != expected || !binder.isBinderAlive) return false
        service = IVirtualDisplayHost.Stub.asInterface(binder)
        return true
    }

    fun current(): IVirtualDisplayHost? {
        val current = service ?: return null
        return try {
            if (current.asBinder().isBinderAlive) current else null
        } catch (_: Throwable) {
            null
        }
    }

    fun clear() {
        service = null
    }
}

internal fun Bundle.putVirtualDisplayHostContainer(value: VirtualDisplayHostBinderContainer) {
    putParcelable(VirtualDisplayHostContract.EXTRA_CONTAINER, value)
}
