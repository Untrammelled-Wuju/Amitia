# Add project specific ProGuard rules here.
# You can control the set of applied configuration files using the
# proguardFiles setting in build.gradle.
#
# For more details, see
#   http://developer.android.com/guide/developing/tools/proguard.html

# Keep Flutter classes
-keep class io.flutter.app.** { *; }
-keep class io.flutter.plugin.**  { *; }
-keep class io.flutter.util.**  { *; }
-keep class io.flutter.view.**  { *; }
-keep class io.flutter.**  { *; }
-keep class io.flutter.plugins.**  { *; }

# Keep native methods
-keepclassmembers class * {
    native <methods>;
}

# Keep Kotlin metadata
-keep class kotlin.Metadata { *; }
-keepclassmembers class **$WhenMappings {
    <fields>;
}

# Keep Parcelable implementations
-keep class * implements android.os.Parcelable {
    public static final android.os.Parcelable$Creator *;
}

# Keep serializable classes
-keepclassmembers class * implements java.io.Serializable {
    static final long serialVersionUID;
    private static final java.io.ObjectStreamField[] serialPersistentFields;
    private void writeObject(java.io.ObjectOutputStream);
    private void readObject(java.io.ObjectInputStream);
    java.lang.Object writeReplace();
    java.lang.Object readResolve();
}

# Google Play Core classes (not available in this project)
-dontwarn com.google.android.play.core.**
-keep class com.google.android.play.core.** { *; }

# Optional AndroidX Window vendor APIs resolved reflectively at runtime
-dontwarn androidx.window.extensions.WindowExtensions
-dontwarn androidx.window.extensions.WindowExtensionsProvider
-dontwarn androidx.window.extensions.area.ExtensionWindowAreaPresentation
-dontwarn androidx.window.extensions.layout.DisplayFeature
-dontwarn androidx.window.extensions.layout.FoldingFeature
-dontwarn androidx.window.extensions.layout.WindowLayoutComponent
-dontwarn androidx.window.extensions.layout.WindowLayoutInfo
-dontwarn androidx.window.sidecar.SidecarDeviceState
-dontwarn androidx.window.sidecar.SidecarDisplayFeature
-dontwarn androidx.window.sidecar.SidecarInterface$SidecarCallback
-dontwarn androidx.window.sidecar.SidecarInterface
-dontwarn androidx.window.sidecar.SidecarProvider
-dontwarn androidx.window.sidecar.SidecarWindowLayoutInfo

# Shizuku classes
-dontwarn dev.rikka.shizuku.**
-keep class dev.rikka.shizuku.** { *; }
-dontwarn rikka.shizuku.**
-keep class rikka.shizuku.** { *; }
-dontwarn rikka.sui.**
-keep class rikka.sui.** { *; }
-keep class com.amitia.amitia_app.nativeprovider.shizuku.ShizukuCommandService { *; }
-keep class com.amitia.amitia_app.nativeprovider.shizuku.IPrivilegedCommandService$Stub { *; }
-keep class com.amitia.amitia_app.virtualdisplay.host.** { *; }
-keep class com.amitia.amitia_app.virtualdisplay.host.IVirtualDisplayHost$Stub { *; }

# Optional manufacturer Push adapters. These classes are compiled only when the
# matching SDK is present, but must keep their manifest callback entry points.
-keep class com.amitia.amitia_app.notifications.vendor.** { *; }
-dontwarn com.xiaomi.mipush.**
-dontwarn com.huawei.hms.**
-dontwarn com.honor.push.**
-keep class com.heytap.msp.** { *; }
-keep class * extends com.heytap.msp.push.service.DataMessageCallbackService { *; }
-keep class * extends com.heytap.msp.push.service.CompatibleDataMessageCallbackService { *; }
-dontwarn com.heytap.msp.**
-keep class com.vivo.push.** { *; }
-keep class * extends com.vivo.push.sdk.OpenClientPushMessageReceiver { *; }
-dontwarn com.vivo.push.**
