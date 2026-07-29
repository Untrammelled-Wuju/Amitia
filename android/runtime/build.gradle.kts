plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.hilt)
    alias(libs.plugins.kapt)
}

android {
    namespace = "com.amitia.runtime"
    compileSdk = 34

    defaultConfig {
        minSdk = 26
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        consumerProguardFiles("consumer-rules.pro")
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
        isCoreLibraryDesugaringEnabled = true
    }

    kotlinOptions {
        jvmTarget = "17"
        freeCompilerArgs += listOf(
            "-opt-in=kotlinx.coroutines.ExperimentalCoroutinesApi",
            "-opt-in=kotlinx.serialization.ExperimentalSerializationApi"
        )
    }

    buildFeatures {
        buildConfig = true
    }

    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            isReturnDefaultValues = true
        }
    }
}

val asciiCacheDir = file("${System.getProperty("user.home")}/.gradle/u-ai-ascii-cache/runtime")

val copyTestClassesToAscii by tasks.registering(Copy::class) {
    from(layout.buildDirectory.dir("intermediates/classes/debugUnitTest/transformDebugUnitTestClassesWithAsm/dirs"))
    into("$asciiCacheDir/testClasses")
    dependsOn("transformDebugUnitTestClassesWithAsm")
}

val packageAsciiClasses by tasks.registering(Jar::class) {
    archiveBaseName = "ascii-all-classes"
    destinationDirectory = file(asciiCacheDir)
    from(layout.buildDirectory.dir("intermediates/classes/debug/transformDebugClassesWithAsm/dirs"))
    from(project(":core").layout.buildDirectory.dir("intermediates/classes/debug/transformDebugClassesWithAsm/dirs"))
    dependsOn("transformDebugClassesWithAsm", ":core:transformDebugClassesWithAsm")
}

afterEvaluate {
    tasks.named<Test>("testDebugUnitTest").configure {
        dependsOn(copyTestClassesToAscii, packageAsciiClasses)
        val asciiTestClassesDir = file("$asciiCacheDir/testClasses")
        val fatJar = packageAsciiClasses.get().archiveFile.get().asFile
        testClassesDirs = files(asciiTestClassesDir)
        classpath = files(asciiTestClassesDir, fatJar) + classpath.filter {
            val path = it.absolutePath
            path.matches(Regex("^[\\x00-\\x7F]+$")) &&
            !path.contains("transformDebugClassesWithAsm") &&
            !path.contains("transformDebugUnitTestClassesWithAsm")
        }
    }
}

dependencies {
    implementation(project(":core"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)

    implementation(libs.google.hilt.android)
    kapt(libs.google.hilt.compiler)

    implementation(libs.squareup.retrofit)
    implementation(libs.squareup.retrofit.kotlinx.serialization)
    implementation(libs.squareup.okhttp)
    implementation(libs.squareup.okhttp.logging)
    implementation(libs.jetbrains.kotlinx.serialization.json)
    implementation(libs.jetbrains.kotlinx.datetime)
    implementation(libs.jetbrains.kotlinx.coroutines.core)

    implementation(libs.androidx.datastore.preferences)

    implementation(libs.androidx.work.runtime.ktx)

    implementation(libs.bouncycastle.provider)

    coreLibraryDesugaring(libs.desugar.jdk.libs)

    testImplementation(libs.junit)
    testImplementation(libs.mockk)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)
    testImplementation(libs.robolectric)
    testImplementation(libs.androidx.test.core)
    testImplementation(libs.androidx.arch.core.testing)
    testImplementation(libs.truth)
    testImplementation(libs.squareup.okhttp)
    testImplementation(libs.okhttp.mockwebserver)

    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.rules)
    androidTestImplementation(libs.mockk.android)
}
