plugins {
    id("com.android.application")
    id("kotlin-android")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

fun amitiaConfigValue(name: String): String =
    providers.gradleProperty(name)
        .orElse(providers.environmentVariable(name))
        .orElse("")
        .get()

fun quotedBuildConfig(value: String): String =
    "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\""

val vendorPushLibDir = file("libs")
val vendorPushArchives =
    vendorPushLibDir.listFiles()
        ?.filter { it.isFile && (it.extension.equals("aar", true) || it.extension.equals("jar", true)) }
        ?.map { it.name.lowercase() }
        .orEmpty()

fun hasVendorPushArchive(vararg hints: String): Boolean =
    vendorPushArchives.any { name -> hints.all { hint -> name.contains(hint.lowercase()) } }

fun amitiaEnabled(name: String): Boolean =
    amitiaConfigValue(name).trim().lowercase() in setOf("1", "true", "yes", "on")

val huaweiPushDependency = amitiaConfigValue("AMITIA_HUAWEI_PUSH_DEPENDENCY").trim()
val honorPushDependency = amitiaConfigValue("AMITIA_HONOR_PUSH_DEPENDENCY").trim()
val oppoPushDependency = amitiaConfigValue("AMITIA_OPPO_PUSH_DEPENDENCY").trim()
val vivoPushDependency = amitiaConfigValue("AMITIA_VIVO_PUSH_DEPENDENCY").trim()
val honorSdkPackage = amitiaConfigValue("AMITIA_HONOR_SDK_PACKAGE")
    .trim()
    .lowercase()
    .ifEmpty { "hihonor" }
val honorLegacyPackage = honorSdkPackage == "honor"
val xiaomiNativeDataEnabled =
    amitiaEnabled("AMITIA_XIAOMI_NATIVE_DATA") || hasVendorPushArchive("mipush")
val huaweiNativeDataEnabled =
    amitiaEnabled("AMITIA_HUAWEI_NATIVE_DATA") ||
        huaweiPushDependency.isNotEmpty() ||
        hasVendorPushArchive("huawei", "push") ||
        hasVendorPushArchive("hms", "push")
val honorNativeDataEnabled =
    amitiaEnabled("AMITIA_HONOR_NATIVE_DATA") ||
        honorPushDependency.isNotEmpty() ||
        hasVendorPushArchive("honor", "push")
val oppoPushSdkLinked =
    oppoPushDependency.isNotEmpty() ||
        hasVendorPushArchive("heytap", "push") ||
        hasVendorPushArchive("oppo", "push") ||
        hasVendorPushArchive("mcs")
val vivoPushSdkLinked =
    vivoPushDependency.isNotEmpty() ||
        hasVendorPushArchive("vivo", "push")
val oppoNativeDataEnabled =
    amitiaEnabled("AMITIA_OPPO_NATIVE_DATA") && oppoPushSdkLinked
val vivoNativeDataEnabled =
    amitiaEnabled("AMITIA_VIVO_NATIVE_DATA") && vivoPushSdkLinked

// Android's native aidl.exe writes dependency files containing absolute input
// paths. On Windows, non-ASCII repository paths may be emitted in the active
// code page while AGP reads them as UTF-8. The regular build directory is on C:
// in this project while the checkout may be on another drive, and AGP's
// SourceDirectorySet cannot relativize cross-drive roots. Stage AIDL on the
// checkout drive itself, under an ASCII-only root-level cache directory.
// A path fingerprint keeps parallel checkouts isolated.
val isWindowsHost = System.getProperty("os.name").lowercase().contains("windows")
val aidlProjectFingerprint =
    Integer.toUnsignedString(layout.projectDirectory.asFile.absolutePath.hashCode(), 16)
val stagedAppAidlRoot =
    if (isWindowsHost) {
        File(
            layout.projectDirectory.asFile.toPath().root.toFile(),
            "amitia-aidl-stage/$aidlProjectFingerprint",
        )
    } else {
        layout.buildDirectory.dir("generated/amitia-aidl").get().asFile
    }
val stagedAppAidlDir = File(stagedAppAidlRoot, "main")
fun stagedVariantAidlDir(name: String) = File(stagedAppAidlRoot, name)
val stageAppAidlSources by tasks.registering(org.gradle.api.tasks.Sync::class) {
    from(layout.projectDirectory.dir("src/main/aidl"))
    into(stagedAppAidlDir)
    doLast {
        // AGP adds build-type AIDL include roots even when the checkout has no
        // matching directory. Keep those implicit roots ASCII-only as well so
        // aidl.exe never emits the non-UTF-8 checkout path into generated Java.
        listOf("debug", "profile", "release").forEach { name ->
            stagedVariantAidlDir(name).mkdirs()
        }
    }
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
        buildConfigField("String", "AMITIA_FCM_APPLICATION_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_FCM_APPLICATION_ID")))
        buildConfigField("String", "AMITIA_FCM_API_KEY", quotedBuildConfig(amitiaConfigValue("AMITIA_FCM_API_KEY")))
        buildConfigField("String", "AMITIA_FCM_PROJECT_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_FCM_PROJECT_ID")))
        buildConfigField("String", "AMITIA_FCM_SENDER_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_FCM_SENDER_ID")))
        buildConfigField("String", "AMITIA_XIAOMI_APP_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_XIAOMI_APP_ID")))
        buildConfigField("String", "AMITIA_XIAOMI_APP_KEY", quotedBuildConfig(amitiaConfigValue("AMITIA_XIAOMI_APP_KEY")))
        buildConfigField("String", "AMITIA_HUAWEI_APP_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_HUAWEI_APP_ID")))
        buildConfigField("String", "AMITIA_HONOR_APP_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_HONOR_APP_ID")))
        buildConfigField("String", "AMITIA_OPPO_APP_KEY", quotedBuildConfig(amitiaConfigValue("AMITIA_OPPO_APP_KEY")))
        buildConfigField("String", "AMITIA_OPPO_APP_SECRET", quotedBuildConfig(amitiaConfigValue("AMITIA_OPPO_APP_SECRET")))
        buildConfigField("String", "AMITIA_VIVO_APP_ID", quotedBuildConfig(amitiaConfigValue("AMITIA_VIVO_APP_ID")))
        buildConfigField("String", "AMITIA_VIVO_API_KEY", quotedBuildConfig(amitiaConfigValue("AMITIA_VIVO_API_KEY")))
        manifestPlaceholders["VIVO_APP_ID"] = amitiaConfigValue("AMITIA_VIVO_APP_ID")
        manifestPlaceholders["VIVO_API_KEY"] = amitiaConfigValue("AMITIA_VIVO_API_KEY")
        manifestPlaceholders["HUAWEI_APP_ID"] = amitiaConfigValue("AMITIA_HUAWEI_APP_ID")
        manifestPlaceholders["HONOR_APP_ID"] = amitiaConfigValue("AMITIA_HONOR_APP_ID")
        manifestPlaceholders["AMITIA_XIAOMI_NATIVE_DATA_ENABLED"] = xiaomiNativeDataEnabled.toString()
        manifestPlaceholders["AMITIA_HUAWEI_NATIVE_DATA_ENABLED"] = huaweiNativeDataEnabled.toString()
        manifestPlaceholders["AMITIA_HONOR_NATIVE_DATA_ENABLED"] = honorNativeDataEnabled.toString()
        manifestPlaceholders["AMITIA_OPPO_NATIVE_DATA_ENABLED"] = oppoNativeDataEnabled.toString()
        manifestPlaceholders["AMITIA_VIVO_NATIVE_DATA_ENABLED"] = vivoNativeDataEnabled.toString()
        buildConfigField("boolean", "AMITIA_OPPO_NATIVE_DATA_ENABLED", oppoNativeDataEnabled.toString())
        buildConfigField("boolean", "AMITIA_VIVO_NATIVE_DATA_ENABLED", vivoNativeDataEnabled.toString())
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

    buildFeatures {
        aidl = true
        buildConfig = true
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
            aidl.setSrcDirs(listOf(stagedAppAidlDir))
            assets.srcDir(layout.buildDirectory.dir("generated/accessibility-provider/assets"))
            if (xiaomiNativeDataEnabled) {
                java.srcDir("src/vendorXiaomi/java")
            }
            if (huaweiNativeDataEnabled) {
                java.srcDir("src/vendorHuawei/java")
            }
            if (honorNativeDataEnabled) {
                java.srcDir(
                    if (honorLegacyPackage) {
                        "src/vendorHonorLegacy/java"
                    } else {
                        "src/vendorHonorHiHonor/java"
                    }
                )
            }
            if (oppoNativeDataEnabled) {
                java.srcDir("src/vendorOppo/java")
            }
            if (vivoNativeDataEnabled) {
                java.srcDir("src/vendorVivo/java")
            }
        }
        getByName("debug") {
            aidl.setSrcDirs(listOf(stagedVariantAidlDir("debug")))
        }
        getByName("profile") {
            aidl.setSrcDirs(listOf(stagedVariantAidlDir("profile")))
        }
        getByName("release") {
            aidl.setSrcDirs(listOf(stagedVariantAidlDir("release")))
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

tasks.configureEach {
    if (name.endsWith("Aidl")) {
        dependsOn(stageAppAidlSources)
    }
}

val frozenRuntimePackagePath: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_PATH")
    ?.trim()
    ?.takeIf { it.isNotEmpty() }
val amitiaRuntimeCandidateBuild: String? = System.getenv("AMITIA_RUNTIME_CANDIDATE_BUILD")
val isCandidateBuild = amitiaRuntimeCandidateBuild == "1"
val allowRuntimelessApk = System.getenv("AMITIA_ALLOW_RUNTIMELESS_APK") == "1"
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
            } else if (allowRuntimelessApk) {
                logger.warn(
                    "copyFrozenRuntimePackage: building without an embedded runtime because " +
                        "AMITIA_ALLOW_RUNTIMELESS_APK=1. This APK supports cloud-only deployment."
                )
            } else {
                throw GradleException(
                    "copyFrozenRuntimePackage: embedded Runtime Package is missing. " +
                        "Set FROZEN_RUNTIME_PACKAGE_PATH for a local-runtime APK, or explicitly set " +
                        "AMITIA_ALLOW_RUNTIMELESS_APK=1 for a cloud-only APK. Refusing to produce an " +
                        "APK that installs successfully but cannot start its local backend on a fresh device."
                )
            }
        }
    }
}

tasks.register("validateBundledRuntimePackage") {
    group = "verification"
    description = "Rejects stale or incomplete embedded Runtime Packages before APK assembly"
    dependsOn("copyFrozenRuntimePackage")

    doLast {
        if (allowRuntimelessApk && !bundledRuntimeAsset.asFile.isFile) {
            logger.lifecycle("validateBundledRuntimePackage: skipped for explicit cloud-only runtimeless APK")
            return@doLast
        }

        val packageFile = bundledRuntimeAsset.asFile
        if (!packageFile.isFile) {
            throw GradleException(
                "validateBundledRuntimePackage: embedded Runtime Package is missing: ${packageFile.absolutePath}"
            )
        }

        val validator = layout.projectDirectory.file(
            "../scripts/validate-runtime-package.py"
        ).asFile
        if (!validator.isFile) {
            throw GradleException(
                "validateBundledRuntimePackage: validator missing: ${validator.absolutePath}"
            )
        }

        val configuredPython = System.getenv("PYTHON")?.trim()?.takeIf { it.isNotEmpty() }
        val python = configuredPython ?: if (
            System.getProperty("os.name").lowercase().contains("windows")
        ) {
            "python.exe"
        } else {
            "python3"
        }

        try {
            val repositoryRoot = rootProject.projectDir.parentFile.parentFile
            val gitExecutable = System.getenv("GIT_BIN")?.trim()?.takeIf { it.isNotEmpty() }
                ?: if (System.getProperty("os.name").lowercase().contains("windows")) {
                    "C:\\Code\\Git\\Git\\bin\\git.exe"
                } else {
                    "git"
                }
            val sourceCommit = providers.exec {
                commandLine(gitExecutable, "-C", repositoryRoot.absolutePath, "rev-parse", "HEAD")
            }.standardOutput.asText.get().trim().lowercase()
            exec {
                commandLine(
                    python,
                    validator.absolutePath,
                    "--package",
                    packageFile.absolutePath,
                    "--expected-source-commit",
                    sourceCommit,
                )
            }
        } catch (error: Exception) {
            throw GradleException(
                "Embedded Runtime Package validation failed. " +
                    "Do not build an APK with a stale source-tree asset. " +
                    "Regenerate it with scripts/build-apk.ps1 (or provide FROZEN_RUNTIME_PACKAGE_PATH). " +
                    "Package=${packageFile.absolutePath}",
                error,
            )
        }
    }
}

tasks.named("preBuild").configure {
    dependsOn("validateBundledRuntimePackage")
    dependsOn("copyAccessibilityProviderAsset")
}

flutter {
    source = "../.."
}

dependencies {
    // Optional vendor push SDKs (for example Xiaomi's official AAR) can be
    // dropped into app/libs without making the default open build depend on
    // proprietary artifacts. Runtime integration is reflection-based.
    implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.aar", "*.jar"))))
    if (huaweiPushDependency.isNotEmpty()) {
        implementation(huaweiPushDependency)
    }
    if (honorPushDependency.isNotEmpty()) {
        implementation(honorPushDependency)
    }
    if (oppoPushDependency.isNotEmpty()) {
        implementation(oppoPushDependency)
    }
    if (vivoPushDependency.isNotEmpty()) {
        implementation(vivoPushDependency)
    }
    implementation(project(":amitia-runtime"))
    implementation("androidx.core:core-ktx:1.17.0")
    implementation("com.google.firebase:firebase-messaging:25.1.3")
    implementation("dev.rikka.shizuku:api:13.1.5")
    implementation("dev.rikka.shizuku:provider:13.1.5")
}

val accessibilityProviderProject = project(":amitia-accessibility")

tasks.register<Copy>("copyAccessibilityProviderAsset") {
    dependsOn(":amitia-accessibility:assembleRelease")
    from(accessibilityProviderProject.layout.buildDirectory.dir("outputs/apk/release")) {
        include("*.apk")
        rename { "amitia-accessibility.apk" }
    }
    into(layout.buildDirectory.dir("generated/accessibility-provider/assets/accessibility"))
}