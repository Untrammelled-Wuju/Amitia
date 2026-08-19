package com.amitia.amitia_app.nativeprovider.shizuku

import android.os.Binder
import android.os.IBinder
import android.os.IInterface
import android.os.Parcel
import android.os.RemoteException

interface IPrivilegedCommandService : IInterface {
    fun executeCommand(requestJson: String): String
    fun destroy()

    companion object {
        const val TRANSACTION_executeCommand: Int = IBinder.FIRST_CALL_TRANSACTION
        const val DESCRIPTOR = "com.amitia.amitia_app.nativeprovider.shizuku.IPrivilegedCommandService"
    }

    abstract class Stub : Binder(), IPrivilegedCommandService {
        companion object {
            fun asInterface(obj: IBinder?): IPrivilegedCommandService? {
                if (obj == null) return null
                val iin = obj.queryLocalInterface(DESCRIPTOR)
                return if (iin is IPrivilegedCommandService) {
                    iin
                } else {
                    Proxy(obj)
                }
            }
        }

        init {
            attachInterface(this, DESCRIPTOR)
        }

        override fun onTransact(code: Int, data: Parcel, reply: Parcel?, flags: Int): Boolean {
            when (code) {
                INTERFACE_TRANSACTION -> {
                    reply?.writeString(DESCRIPTOR)
                    return true
                }
                TRANSACTION_executeCommand -> {
                    data.enforceInterface(DESCRIPTOR)
                    val requestJson: String = data.readString() ?: ""
                    val result = executeCommand(requestJson)
                    reply?.writeNoException()
                    reply?.writeString(result)
                    return true
                }
            }
            return false
        }

        abstract override fun executeCommand(requestJson: String): String

        override fun destroy() {
        }
    }

    class Proxy(private val mRemote: IBinder) : IPrivilegedCommandService {
        override fun asBinder(): IBinder = mRemote

        override fun executeCommand(requestJson: String): String {
            val data = Parcel.obtain()
            val reply = Parcel.obtain()
            return try {
                data.writeInterfaceToken("com.amitia.amitia_app.nativeprovider.shizuku.IPrivilegedCommandService")
                data.writeString(requestJson)
                mRemote.transact(TRANSACTION_executeCommand, data, reply, 0)
                reply.readException()
                reply.readString() ?: ""
            } catch (e: RemoteException) {
                """{"error":{"code":"REMOTE_EXCEPTION","message":"${e.message?.replace("\"", "\\\"") ?: "remote exception"}"}}"""
            } finally {
                reply.recycle()
                data.recycle()
            }
        }

        override fun destroy() {
            val data = Parcel.obtain()
            try {
                data.writeInterfaceToken("com.amitia.amitia_app.nativeprovider.shizuku.IPrivilegedCommandService")
                mRemote.transact(IBinder.LAST_CALL_TRANSACTION, data, null, IBinder.FLAG_ONEWAY)
            } catch (_: RemoteException) {
            } finally {
                data.recycle()
            }
        }
    }
}
