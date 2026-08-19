import java.security.MessageDigest

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

    sourceSets {
        getByName("main") {
            aidl.srcDirs("src/main/aidl")
        }
    }
}

val frozenRuntimePackagePath: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_PATH")
val frozenRuntimePackageSha256: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_SHA256")
val amitiaRuntimeCandidateBuild: String? = System.getenv("AMITIA_RUNTIME_CANDIDATE_BUILD")
val isCandidateBuild = amitiaRuntimeCandidateBuild == "1"

if (isCandidateBuild && frozenRuntimePackagePath == null) {
    throw GradleException("Candidate build requires FROZEN_RUNTIME_PACKAGE_PATH environment variable")
}
if (isCandidateBuild && frozenRuntimePackageSha256 == null) {
    throw GradleException("Candidate build requires FROZEN_RUNTIME_PACKAGE_SHA256 environment variable")
}

tasks.register<Delete>("cleanFrozenRuntimePackage") {
    group = "candidate"
    description = "Removes previously copied frozen Runtime Package from assets"
    delete(layout.projectDirectory.dir("src/main/assets/runtime-package"))
}

tasks.register<Copy>("copyFrozenRuntimePackage") {
    group = "candidate"
    description = "Copies the Step 7 frozen Runtime Package into APK assets"
    dependsOn("cleanFrozenRuntimePackage")
    if (frozenRuntimePackagePath != null && frozenRuntimePackageSha256 != null) {
        val sourceFile = file(frozenRuntimePackagePath)
        if (!sourceFile.exists()) {
            throw GradleException("copyFrozenRuntimePackage: FROZEN_RUNTIME_PACKAGE_PATH declared but file missing: $frozenRuntimePackagePath")
        }
        doFirst {
            val actualSha = sourceFile.inputStream().use { input ->
                val digest = MessageDigest.getInstance("SHA-256")
                val buffer = ByteArray(8192)
                var read: Int
                while (input.read(buffer).also { read = it } != -1) {
                    digest.update(buffer, 0, read)
                }
                digest.digest().joinToString("") { it.toInt().and(0xFF).toString(16).padStart(2, '0') }
            }
            if (!actualSha.equals(frozenRuntimePackageSha256, ignoreCase = true)) {
                throw GradleException("copyFrozenRuntimePackage: SHA256 mismatch for $frozenRuntimePackagePath: expected=$frozenRuntimePackageSha256 actual=$actualSha")
            }
        }
        from(sourceFile) {
            rename { "amitia-runtime-1.0.0.zip" }
        }
        into(layout.projectDirectory.dir("src/main/assets/runtime-package"))
    } else if (frozenRuntimePackagePath != null || frozenRuntimePackageSha256 != null) {
        throw GradleException("copyFrozenRuntimePackage: Both FROZEN_RUNTIME_PACKAGE_PATH and FROZEN_RUNTIME_PACKAGE_SHA256 must be set for Candidate build")
    } else if (isCandidateBuild) {
        throw GradleException("copyFrozenRuntimePackage: Candidate build requires FROZEN_RUNTIME_PACKAGE_PATH and FROZEN_RUNTIME_PACKAGE_SHA256")
    } else {
        doLast {
            logger.lifecycle("copyFrozenRuntimePackage: Candidate environment not set, skipping asset embed")
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
    implementation("dev.rikka.shizuku:api:13.1.5")
    implementation("dev.rikka.shizuku:provider:13.1.5")
}
