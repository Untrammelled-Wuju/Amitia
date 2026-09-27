plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
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
            aidl.srcDirs("src/main/aidl")
        }
    }

    base {
        archivesName.set("amitia-accessibility")
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
}
