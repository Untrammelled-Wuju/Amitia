import org.gradle.api.tasks.testing.Test

plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.kotlin.android) apply false
    alias(libs.plugins.kotlin.compose) apply false
    alias(libs.plugins.kotlin.serialization) apply false
    alias(libs.plugins.hilt) apply false
    alias(libs.plugins.kapt) apply false
}

subprojects {
    tasks.withType<Test>().configureEach {
        environment("JAVA_TOOL_OPTIONS", "-Dfile.encoding=UTF-8 -Dsun.jnu.encoding=UTF-8")
        jvmArgs("-Dfile.encoding=UTF-8", "-Dsun.jnu.encoding=UTF-8")
    }
}
