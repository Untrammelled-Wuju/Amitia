val sourcePath = rootProject.projectDir.absolutePath
val usesNonAsciiPath = sourcePath.any { it.code > 127 }
val isWindowsHost = System.getProperty("os.name").lowercase().contains("windows")
val redirectedBuildRoot = providers.environmentVariable("AMITIA_ANDROID_BUILD_ROOT")
    .orElse(
        providers.provider {
            if (isWindowsHost && usesNonAsciiPath) {
                val tempRoot = System.getenv("LOCALAPPDATA")
                    ?.takeIf { it.isNotBlank() }
                    ?: System.getProperty("java.io.tmpdir")
                java.io.File(tempRoot, "Amitia/android-build").absolutePath
            } else {
                ""
            }
        }
    )
    .get()
    .trim()

if (redirectedBuildRoot.isNotEmpty()) {
    rootProject.layout.buildDirectory.set(file("$redirectedBuildRoot/root"))
}

allprojects {
    buildscript {
        repositories {
            maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") }
            maven { url = uri("https://maven.aliyun.com/repository/google") }
            maven { url = uri("https://maven.aliyun.com/repository/public") }
            google()
            mavenCentral()
            gradlePluginPortal()
        }
    }
    repositories {
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        google()
        mavenCentral()
        maven { url = uri("https://developer.hihonor.com/repo/") }
        maven { url = uri("https://developer.huawei.com/repo/") }
        maven { url = uri("https://storage.flutter-io.cn/download.flutter.io") }
    }
}

subprojects {
    if (redirectedBuildRoot.isNotEmpty()) {
        layout.buildDirectory.set(file("$redirectedBuildRoot/${project.name}"))
    }
    project.evaluationDependsOn(":app")
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}

