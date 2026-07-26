cmake_minimum_required(VERSION 3.22.1)

project(proot_jni C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_library(proot SHARED
    proot_jni.cpp
)

find_library(log-lib log)
find_library(android-lib android)

target_link_libraries(proot
    ${log-lib}
    ${android-lib}
)
