allprojects {
    repositories {
        google()
        mavenCentral()
        maven { url = uri("https://storage.flutter-io.cn/download.flutter.io") }
    }
}

val newBuildDir: Directory = layout.buildDirectory
    .dir("D:/build/${rootProject.name}")
    .get()
rootProject.layout.buildDirectory.value(newBuildDir)

subprojects {
    val newSubprojectBuildDir: Directory = newBuildDir.dir(project.name)
    project.layout.buildDirectory.value(newSubprojectBuildDir)
}
subprojects {
    project.evaluationDependsOn(":app")
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}

