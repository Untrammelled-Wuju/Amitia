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

    val keystorePath: String? = System.getenv("AMITIA_KEYSTORE_PATH")?.trim()?.takeIf { it.isNotEmpty() }
    val keystorePassword: String? = System.getenv("AMITIA_KEYSTORE_PASSWORD")?.takeIf { it.isNotEmpty() }
    val keyAliasValue: String? = System.getenv("AMITIA_KEY_ALIAS")?.trim()?.takeIf { it.isNotEmpty() }
    val keyPasswordValue: String? = System.getenv("AMITIA_KEY_PASSWORD")?.takeIf { it.isNotEmpty() }

    signingConfigs {
        create("release") {
            if (keystorePath != null) {
                storeFile = file(keystorePath)
            }
            if (keystorePassword != null) {
                storePassword = keystorePassword
            }
            if (keyAliasValue != null) {
                keyAlias = keyAliasValue
            }
            if (keyPasswordValue != null) {
                keyPassword = keyPasswordValue
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            isDebuggable = true
        }
        release {
            // Never silently fall back to the debug certificate. Validation below
            // aborts every release build unless all production signing inputs exist.
            signingConfig = signingConfigs.getByName("release")
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


val releaseSigningEnvironment = mapOf(
    "AMITIA_KEYSTORE_PATH" to System.getenv("AMITIA_KEYSTORE_PATH")?.trim(),
    "AMITIA_KEYSTORE_PASSWORD" to System.getenv("AMITIA_KEYSTORE_PASSWORD"),
    "AMITIA_KEY_ALIAS" to System.getenv("AMITIA_KEY_ALIAS")?.trim(),
    "AMITIA_KEY_PASSWORD" to System.getenv("AMITIA_KEY_PASSWORD"),
)

val validateReleaseSigning by tasks.registering {
    group = "verification"
    description = "Fails closed when production Android signing credentials are missing or invalid"
    doLast {
        val missing = releaseSigningEnvironment
            .filterValues { it.isNullOrBlank() }
            .keys
            .sorted()
        if (missing.isNotEmpty()) {
            throw GradleException(
                "Release signing is not configured. Missing: ${missing.joinToString(", ")}. " +
                    "Debug signing fallback is intentionally disabled."
            )
        }

        val configuredKeystore = file(releaseSigningEnvironment.getValue("AMITIA_KEYSTORE_PATH")!!)
        if (!configuredKeystore.isFile) {
            throw GradleException(
                "AMITIA_KEYSTORE_PATH does not point to a readable keystore file: ${configuredKeystore.absolutePath}"
            )
        }
    }
}

tasks.matching { it.name == "preReleaseBuild" }.configureEach {
    dependsOn(validateReleaseSigning)
}

val frozenRuntimePackagePath: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_PATH")
    ?.trim()
    ?.takeIf { it.isNotEmpty() }
val amitiaRuntimeCandidateBuild: String? = System.getenv("AMITIA_RUNTIME_CANDIDATE_BUILD")
val isCandidateBuild = amitiaRuntimeCandidateBuild == "1"
val bundledRuntimeAssetDir = layout.projectDirectory.dir("src/main/assets/runtime-package")
val bundledRuntimeAsset = bundledRuntimeAssetDir.file("amitia-runtime-1.0.0.zip")

tasks.register<Delete>("cleanFrozenRuntimePackage") {
    group = "candidate"
    description = "Removes the generated frozen Runtime Package before replacing it"
    // Ordinary debug/dev builds must never erase a previously bundled runtime.
    onlyIf { frozenRuntimePackagePath != null }
    delete(bundledRuntimeAssetDir)
}

tasks.register<Copy>("copyFrozenRuntimePackage") {
    group = "candidate"
    description = "Copies the configured frozen Runtime Package into APK assets"

    if (frozenRuntimePackagePath != null) {
        dependsOn("cleanFrozenRuntimePackage")
        val sourceFile = file(frozenRuntimePackagePath)
        if (!sourceFile.isFile) {
            throw GradleException(
                "copyFrozenRuntimePackage: FROZEN_RUNTIME_PACKAGE_PATH declared but file missing: $frozenRuntimePackagePath"
            )
        }
        from(sourceFile) {
            rename { "amitia-runtime-1.0.0.zip" }
        }
        into(bundledRuntimeAssetDir)
    } else if (isCandidateBuild) {
        throw GradleException(
            "copyFrozenRuntimePackage: Candidate build requires FROZEN_RUNTIME_PACKAGE_PATH"
        )
    } else {
        doLast {
            if (bundledRuntimeAsset.asFile.isFile) {
                logger.lifecycle(
                    "copyFrozenRuntimePackage: preserving existing bundled runtime asset: ${bundledRuntimeAsset.asFile.absolutePath}"
                )
            } else {
                logger.warn(
                    "copyFrozenRuntimePackage: no bundled runtime asset is present. " +
                        "Fresh-device installation requires FROZEN_RUNTIME_PACKAGE_PATH when building the APK."
                )
            }
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
