plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// AIDL code generation on Windows must not read non-ASCII absolute paths:
// aidl.exe writes dependency paths in the active code page while AGP reads
// those files as UTF-8. Stage on the checkout's own drive, outside buildDir.
val accessibilityWindowsHost = System.getProperty("os.name").lowercase().contains("windows")
val accessibilityAidlHash =
    Integer.toUnsignedString(layout.projectDirectory.asFile.absolutePath.hashCode(), 16)
val accessibilityAidlRoot =
    if (accessibilityWindowsHost) {
        File(
            layout.projectDirectory.asFile.toPath().root.toFile(),
            "amitia-aidl-stage/$accessibilityAidlHash",
        )
    } else {
        layout.buildDirectory.dir("generated/amitia-aidl").get().asFile
    }
val accessibilityAidlMain = File(accessibilityAidlRoot, "main")
fun accessibilityAidlVariant(name: String) = File(accessibilityAidlRoot, name)
val stageAccessibilityAidl by tasks.registering(org.gradle.api.tasks.Sync::class) {
    from(layout.projectDirectory.dir("src/main/aidl"))
    into(accessibilityAidlMain)
    doLast {
        listOf("debug", "profile", "release").forEach { name ->
            accessibilityAidlVariant(name).mkdirs()
        }
    }
}

android {
    namespace = "com.amitia.amitia_app.accessibility"
    compileSdk = 36

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlin {
        compilerOptions {
            jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
        }
    }

    buildFeatures {
        aidl = true
    }

    defaultConfig {
        applicationId = "com.amitia.amitia_app.accessibility"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "1.0.0"
    }

    val keystorePath: String? = System.getenv("AMITIA_KEYSTORE_PATH")?.trim()?.takeIf { it.isNotEmpty() }
    val keystorePassword: String? = System.getenv("AMITIA_KEYSTORE_PASSWORD")?.takeIf { it.isNotEmpty() }
    val keyAliasValue: String? = System.getenv("AMITIA_KEY_ALIAS")?.trim()?.takeIf { it.isNotEmpty() }
    val keyPasswordValue: String? = System.getenv("AMITIA_KEY_PASSWORD")?.takeIf { it.isNotEmpty() }

    signingConfigs {
        create("release") {
            if (keystorePath != null) storeFile = file(keystorePath)
            if (keystorePassword != null) storePassword = keystorePassword
            if (keyAliasValue != null) keyAlias = keyAliasValue
            if (keyPasswordValue != null) keyPassword = keyPasswordValue
        }
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.getByName("release")
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    sourceSets {
        getByName("main") {
            aidl.setSrcDirs(listOf(accessibilityAidlMain))
        }
        getByName("debug") {
            aidl.setSrcDirs(listOf(accessibilityAidlVariant("debug")))
        }
        getByName("release") {
            aidl.setSrcDirs(listOf(accessibilityAidlVariant("release")))
        }
    }

    base {
        archivesName.set("amitia-accessibility")
    }
}

tasks.configureEach {
    if (name.startsWith("compile") && name.endsWith("Aidl")) {
        dependsOn(stageAccessibilityAidl)
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
}
