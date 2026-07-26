#include <jni.h>
#include <android/log.h>

#define PROOT_TAG "AmitiaProot"
#define PROOT_LOGI(...) __android_log_print(ANDROID_LOG_INFO,  PROOT_TAG, __VA_ARGS__)
#define PROOT_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, PROOT_TAG, __VA_ARGS__)

extern "C" {

JNIEXPORT jint JNICALL
Java_com_amitia_nativeproot_ProotBridge_prootMain(JNIEnv* env, jobject thiz, jobjectArray args) {
    if (env == nullptr || args == nullptr) {
        PROOT_LOGE("prootMain called with null arguments");
        return -1;
    }
    jsize argc = env->GetArrayLength(args);
    PROOT_LOGI("prootMain stub invoked, argc=%d", static_cast<int>(argc));
    return 0;
}

JNIEXPORT jstring JNICALL
Java_com_amitia_nativeproot_ProotBridge_prootVersion(JNIEnv* env, jobject thiz) {
    if (env == nullptr) {
        return nullptr;
    }
    const char* version = "proot-jni-stub-0.1.0";
    return env->NewStringUTF(version);
}

JNIEXPORT void JNICALL
Java_com_amitia_nativeproot_ProotBridge_prootSetTrace(JNIEnv* env, jobject thiz, jboolean enabled) {
    PROOT_LOGI("prootSetTrace stub invoked, enabled=%d", enabled ? 1 : 0);
}

JNIEXPORT jint JNICALL
Java_com_amitia_nativeproot_ProotBridge_prootShutdown(JNIEnv* env, jobject thiz) {
    PROOT_LOGI("prootShutdown stub invoked");
    return 0;
}

}
