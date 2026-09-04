import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import java.security.MessageDigest

plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.amitia.amitia_app.runtime"
    compileSdk = 35

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        buildConfig = true
        resValues = false
        aidl = false
    }

    defaultConfig {
        minSdk = 21
        ndk {
            abiFilters.clear()
            abiFilters.add("arm64-v8a")
        }

        val frozenRuntimePackagePath: String? = System.getenv("FROZEN_RUNTIME_PACKAGE_PATH")
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
        val explicitRuntimePackage = frozenRuntimePackagePath?.let { file(it) }
        val preservedBundledAsset = file(
            "../app/src/main/assets/runtime-package/amitia-runtime-1.0.0.zip"
        )
        val runtimePackageForHash = when {
            explicitRuntimePackage?.isFile == true -> explicitRuntimePackage
            preservedBundledAsset.isFile -> preservedBundledAsset
            else -> null
        }
        val runtimeSha = if (runtimePackageForHash != null) {
            runtimePackageForHash.inputStream().use { input ->
                val digest = MessageDigest.getInstance("SHA-256")
                val buffer = ByteArray(8192)
                var read: Int
                while (input.read(buffer).also { read = it } != -1) {
                    digest.update(buffer, 0, read)
                }
                digest.digest().joinToString("") {
                    it.toInt().and(0xFF).toString(16).padStart(2, '0')
                }
            }
        } else {
            "BUNDLED_RUNTIME_PACKAGE_MISSING"
        }
        buildConfigField("String", "RUNTIME_PACKAGE_SHA256", "\"$runtimeSha\"")
    }

    sourceSets {
        getByName("main") {
            jniLibs.srcDir("src/main/jniLibs")
            jniLibs.srcDir(layout.buildDirectory.dir("generated/jniLibs"))
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

tasks.withType<Test>().configureEach {
    testLogging {
        showStandardStreams = true
        exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
    }
    jvmArgs(
        "--add-opens=java.base/java.lang=ALL-UNNAMED",
        "--add-opens=java.base/java.lang.reflect=ALL-UNNAMED",
        "--add-opens=java.base/java.util=ALL-UNNAMED",
        "--add-opens=java.base/java.io=ALL-UNNAMED",
        "--add-opens=java.base/jdk.internal.loader=ALL-UNNAMED"
    )
}

val prootArmDir = layout.projectDirectory.dir("src/main/jniLibs/arm64-v8a")
val prootArmFile = layout.projectDirectory.file("src/main/jniLibs/arm64-v8a/libamitia_proot.so")
val prootMetadataFile = layout.projectDirectory.file("src/main/res/raw/proot_artifact.json")
val androidBackendInput = providers.gradleProperty("amitiaAndroidBackendArm64").orNull?.let(::file)
val androidBackendOutput = layout.buildDirectory.file("generated/jniLibs/arm64-v8a/libamitia_server.so")

// PRoot mode launches the backend from the installed rootfs
// (versionsRoot/<version>/backend/amitia-server), so no native .so is required.
// Android-native builds must still opt in explicitly by passing
// -PamitiaAndroidBackendArm64=<android arm64 backend>.
tasks.register<Copy>("stageAndroidBackend") {
    val input = androidBackendInput
    onlyIf { input != null }
    if (input != null) {
        from(input)
        into(androidBackendOutput.get().asFile.parentFile)
        rename { "libamitia_server.so" }
        inputs.file(input)
        outputs.file(androidBackendOutput)
    }
}

tasks.register("verifyProotArtifact") {
    group = "verification"
    description = "Verifies PRoot arm64-v8a artifact, metadata, and ABI contract"
    doLast {
        if (!prootArmFile.asFile.exists()) throw GradleException("verifyProotArtifact FAILED: arm64-v8a/libamitia_proot.so not found")
        if (prootArmFile.asFile.length() == 0L) throw GradleException("verifyProotArtifact FAILED: arm64-v8a/libamitia_proot.so is empty")
        listOf("x86", "x86_64", "armeabi-v7a", "armeabi", "mips", "mips64").forEach { abi ->
            val dir = layout.projectDirectory.dir("src/main/jniLibs/$abi").asFile
            if (dir.exists()) throw GradleException("verifyProotArtifact FAILED: forbidden jniLibs directory exists: $abi")
        }
        if (!prootMetadataFile.asFile.exists()) throw GradleException("verifyProotArtifact FAILED: proot_artifact.json not found")
        val metadata = prootMetadataFile.asFile.readText()
        val json = groovy.json.JsonSlurper().parseText(metadata) as Map<*, *>
        if (json["abi"] != "arm64-v8a") throw GradleException("verifyProotArtifact FAILED: metadata abi must be arm64-v8a, got: ${json["abi"]}")
        if (json["architecture"] != "aarch64") throw GradleException("verifyProotArtifact FAILED: metadata architecture must be aarch64, got: ${json["architecture"]}")
        if (json["fileName"] != "libamitia_proot.so") throw GradleException("verifyProotArtifact FAILED: metadata fileName must be libamitia_proot.so, got: ${json["fileName"]}")
        if (json["componentId"] != "runtime.proot") throw GradleException("verifyProotArtifact FAILED: metadata componentId must be runtime.proot, got: ${json["componentId"]}")
        if ((json["schemaVersion"] as? Int) != 1) throw GradleException("verifyProotArtifact FAILED: metadata schemaVersion must be 1, got: ${json["schemaVersion"]}")
        val metadataSha = json["sha256"] as? String ?: throw GradleException("verifyProotArtifact FAILED: metadata sha256 missing")
        if (!metadataSha.matches(Regex("^[0-9a-f]{64}$"))) throw GradleException("verifyProotArtifact FAILED: metadata sha256 invalid format")
        val source = json["source"] as? Map<*, *>
        if (source != null) {
            val patchCommit = source["androidPatchCommit"] as? String ?: ""
            if (patchCommit.contains("placeholder")) throw GradleException("verifyProotArtifact FAILED: patch provenance is placeholder")
        }
        val sha256 = MessageDigest.getInstance("SHA-256")
        prootArmFile.asFile.inputStream().use { fis ->
            val buf = ByteArray(8192)
            while (true) { val r = fis.read(buf); if (r == -1) break; sha256.update(buf, 0, r) }
        }
        val actualSha = sha256.digest().joinToString("") { b -> "%02x".format(b) }
        if (actualSha != metadataSha) throw GradleException("verifyProotArtifact FAILED: SHA256 mismatch (metadata=$metadataSha, actual=$actualSha)")
        val header = ByteArray(64)
        prootArmFile.asFile.inputStream().use { it.read(header) }
        if (header[0] != 0x7F.toByte() || header[1] != 'E'.code.toByte() || header[2] != 'L'.code.toByte() || header[3] != 'F'.code.toByte())
            throw GradleException("verifyProotArtifact FAILED: not an ELF binary")
        if (header[4].toInt() and 0xFF != 2) throw GradleException("verifyProotArtifact FAILED: not ELF64")
        val machine = (header[18].toInt() and 0xFF) or ((header[19].toInt() and 0xFF) shl 8)
        if (machine != 183) throw GradleException("verifyProotArtifact FAILED: not AArch64 (machine=$machine)")
        logger.lifecycle("verifyProotArtifact PASSED: arm64-v8a/libamitia_proot.so verified (SHA256=$actualSha, ELF64 AArch64)")
    }
}

tasks.named("preBuild").configure {
    dependsOn("verifyProotArtifact")
    dependsOn("stageAndroidBackend")
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("org.apache.commons:commons-compress:1.27.0")
    implementation("org.tukaani:xz:1.10")
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.robolectric:robolectric:4.13")
    testImplementation("androidx.test:core:1.6.1")
    testImplementation("androidx.test.ext:junit:1.2.1")
}
