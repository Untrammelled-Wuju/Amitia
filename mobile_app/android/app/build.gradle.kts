plugins {
    id("com.android.application")
    id("kotlin-android")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

android {
    namespace = "com.amitia.amitia_app"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlin {
        compilerOptions {
            jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
        }
    }

    defaultConfig {
        // TODO: Specify your own unique Application ID (https://developer.android.com/studio/build/application-id.html).
        applicationId = "com.amitia.amitia_app"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
        ndk {
            abiFilters.clear()
            abiFilters.add("arm64-v8a")
        }
    }

    val keystorePath: String? = System.getenv("AMITIA_KEYSTORE_PATH")
    val keystorePassword: String? = System.getenv("AMITIA_KEYSTORE_PASSWORD")
    val keyAliasValue: String? = System.getenv("AMITIA_KEY_ALIAS")
    val keyPasswordValue: String? = System.getenv("AMITIA_KEY_PASSWORD")
    val hasReleaseKeystore = keystorePath != null && file(keystorePath).exists()

    signingConfigs {
        create("release") {
            if (hasReleaseKeystore) {
                storeFile = file(keystorePath!!)
                storePassword = keystorePassword ?: ""
                keyAlias = keyAliasValue ?: "amitia"
                keyPassword = keyPasswordValue ?: ""
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            isDebuggable = true
        }
        release {
            signingConfig = if (hasReleaseKeystore) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("debug")
            }
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    packaging {
        jniLibs {
            useLegacyPackaging = true
            pickFirsts.add("**/libc++_shared.so")
            keepDebugSymbols.add("**/libamitia_proot.so")
            excludes.add("lib/armeabi-v7a/**")
            excludes.add("lib/x86_64/**")
            excludes.add("lib/x86/**")
            excludes.add("lib/armeabi/**")
        }
    }
}

val frozenRuntimePackagePath: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_PATH")
val frozenRuntimePackageSha256: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_SHA256")

tasks.register<Delete>("cleanFrozenRuntimePackage") {
    group = "candidate"
    description = "Removes previously copied frozen Runtime Package from assets"
    delete(layout.projectDirectory.dir("src/main/assets/runtime-package"))
}

tasks.register<Copy>("copyFrozenRuntimePackage") {
    group = "candidate"
    description = "Copies the Step 7 frozen Runtime Package into APK assets"
    dependsOn("cleanFrozenRuntimePackage")
    if (frozenRuntimePackagePath != null && file(frozenRuntimePackagePath).exists()) {
        from(file(frozenRuntimePackagePath)) {
            rename { "amitia-runtime-1.0.0.zip" }
        }
        into(layout.projectDirectory.dir("src/main/assets/runtime-package"))
    } else {
        doLast {
            logger.lifecycle("copyFrozenRuntimePackage: FROZEN_RUNTIME_PACKAGE_PATH not set or file missing, skipping asset embed")
        }
    }
}

tasks.named("preBuild").configure {
    dependsOn("copyFrozenRuntimePackage")
}

flutter {
    source = "../.."
}

dependencies {
    implementation(project(":amitia-runtime"))
}
