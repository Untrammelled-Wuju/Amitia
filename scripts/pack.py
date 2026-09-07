# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
import os
import subprocess
import sys

import tempfile
import shutil

workspace = os.path.normpath(r'D:\桌面\跟进项目\U-Ai')
output = os.path.normpath(r'D:\桌面\跟进项目\U-Ai\U-Ai-source.tar.gz')
folder_name = os.path.basename(workspace)
parent_dir = os.path.dirname(workspace)
temp_output = os.path.join(tempfile.gettempdir(), 'U-Ai-source.tar.gz')

if os.path.exists(output):
    os.remove(output)
if os.path.exists(temp_output):
    os.remove(temp_output)

excludes = [
    '--exclude=node_modules',
    '--exclude=.git',
    '--exclude=*.exe',
    '--exclude=*.log',
    '--exclude=desktop/dist',
    '--exclude=desktop/build',
    '--exclude=desktop/release',
    '--exclude=front/dist',
    '--exclude=mobile_app/build',
    '--exclude=mobile_app/android/app/build',
    '--exclude=mobile_app/android/amitia-runtime/build',
    '--exclude=mobile_app/android/.gradle',
    '--exclude=' + folder_name + '/mobile_app/windows/flutter/ephemeral',
    '--exclude=' + folder_name + '/mobile_app/.flutter-plugins-dependencies',
    '--exclude=' + folder_name + '/mobile_app/android/local.properties',
    '--exclude=' + folder_name + '/mobile_app/android/.cxx',
    '--exclude=' + folder_name + '/mobile_app/android/app/.cxx',
    '--exclude=' + folder_name + '/mobile_app/android/amitia-runtime/.cxx',
    '--exclude=' + folder_name + '/mobile_app/android/build',
    '--exclude=' + folder_name + '/mobile_app/android/.kotlin',
    '--exclude=' + folder_name + '/mobile_app/ios/Flutter/ephemeral',
    '--exclude=' + folder_name + '/mobile_app/linux/flutter/ephemeral',
    '--exclude=' + folder_name + '/mobile_app/macos/Flutter/ephemeral',
    '--exclude=backend/pkg/gameplugin/sdk/game-plugin/dist',
    '--exclude=*.db',
    '--exclude=*.db-shm',
    '--exclude=*.db-wal',
    '--exclude=*.db-journal',
    '--exclude=qdrant/storage',
    '--exclude=surrealdb/surreal.exe',
    '--exclude=surrealdb-data-temp',
    '--exclude=desktop/dist-types',
    '--exclude=desktop/resources/core',
    '--exclude=sdk/plugin-sdk/dist',
    '--exclude=sdk/plugin-sdk/node_modules',
    '--exclude=*.pyc',
    '--exclude=__pycache__',
    '--exclude=.DS_Store',
    '--exclude=Thumbs.db',
    '--exclude=*.bak',
    '--exclude=*.tmp',
    '--exclude=*.orig',
    '--exclude=.vscode',
    '--exclude=.idea',
    '--exclude=.env',
    '--exclude=.env.local',
    '--exclude=.publish-config.json',
    '--exclude=backend/data',
    '--exclude=backend/cmd/data',
    '--exclude=' + folder_name + '/data',
    '--exclude=logs',
    '--exclude=runtime/out',
    '--exclude=backend/server_linux_amd64',
    '--exclude=backend/server_linux_arm64',
    '--exclude=backend/server',
    '--exclude=backend/surrealdb/surreal.zip',
    '--exclude=backend/qdrant/qdrant.zip',
    '--exclude=backend/node/node.exe.zip',
    '--exclude=desktop/resources/qdrant/qdrant.zip',
    '--exclude=desktop/resources/surrealdb/surrealdb/surreal.zip',
    '--exclude=desktop/resources/surrealdb/surreal.zip',
    '--exclude=desktop/resources/core/node/node.zip',
    '--exclude=mobile_app/android/app/src/main/assets/runtime-package',
    '--exclude=backend/server.exe',
    '--exclude=backend/server.exe~',
    '--exclude=backend/cmd/server/server.exe',
    '--exclude=backend/cmd/server/backend.exe',
    '--exclude=backend/cmd/server/backend',
    '--exclude=backend/amitia-ext.exe',
    '--exclude=backend/amitiax.exe',
    '--exclude=backend/extension.test.exe',
    '--exclude=backend/kernel.test.exe',
    '--exclude=backend/legacy-package-migrate.exe',
    '--exclude=backend/worker.test.exe',
    '--exclude=backend/server_*.exe',
    '--exclude=*.tar',
    '--exclude=*.tar.gz',
    '--exclude=*.tar.xz',
    '--exclude=*.zip',
    '--exclude=artifacts',
    '--exclude=.dart_tool',
    '--exclude=temp_extract_integration',
    '--exclude=AmitiaData',
    '--exclude=U-Ai-source.tar.gz',
    '--exclude=installed-runtime-package.zip',
    '--exclude=amitia-runtime-root-debug.tar.xz',
    '--exclude=rootfs-seed-debug.tar.xz',
    '--exclude=.codex-temp-patches',
    '--exclude=.codex-diff-apply',
    '--exclude=classes*.dex.txt',
]

cmd = ['tar', '-czf', temp_output] + excludes + [folder_name]

print(f'Packing {folder_name} -> {output}')
result = subprocess.run(cmd, cwd=parent_dir, capture_output=True, text=True)

if result.stdout:
    print(result.stdout)
if result.stderr:
    print(f'stderr: {result.stderr}')

print(f'returncode: {result.returncode}')

if os.path.exists(temp_output):
    size = os.path.getsize(temp_output)
    print(f'Archive created: {size/1024/1024:.1f} MB')
    
    # Move to final location
    shutil.move(temp_output, output)
    print(f'Moved to: {output}')
    
    # Verify with system tar
    verify = subprocess.run(['tar', '-tzf', output], capture_output=True, text=True)
    if verify.returncode == 0:
        lines = verify.stdout.strip().split('\n')
        print(f'Verified: {len(lines)} entries')
        gc = [l for l in lines if 'game_center_api' in l]
        print(f'game_center_api.dart: {len(gc) > 0}')
    else:
        print(f'Verify error: {verify.stderr}')
else:
    print('Archive NOT created')
