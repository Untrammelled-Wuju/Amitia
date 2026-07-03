# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
import ctypes
import ctypes.wintypes as wt
import subprocess
import os
import sys
import time


PAGE_READWRITE = 0x04
MEM_COMMIT = 0x1000
MEM_RESERVE = 0x2000
PROCESS_ALL_ACCESS = 0x1F0FFF


def find_wechat_pid():
    r = subprocess.run(
        ["tasklist", "/FI", "IMAGENAME eq Weixin.exe", "/FO", "CSV", "/NH"],
        capture_output=True, text=True
    )
    pids = []
    for line in r.stdout.strip().split("\n"):
        if not line.strip():
            continue
        p = line.strip('"').split('","')
        if len(p) >= 5:
            pid = int(p[1])
            mem = int(p[4].replace(",", "").replace(" K", "").strip() or "0")
            pids.append((pid, mem))
    if not pids:
        raise RuntimeError("Weixin.exe 未运行")
    pids.sort(key=lambda x: x[1], reverse=True)
    return pids[0][0]


def inject_dll(pid, dll_path):
    dll_path_bytes = dll_path.encode("utf-8")
    dll_path_len = len(dll_path_bytes) + 1

    kernel32 = ctypes.windll.kernel32

    h_process = kernel32.OpenProcess(PROCESS_ALL_ACCESS, False, pid)
    if not h_process:
        raise RuntimeError(f"无法打开进程 PID={pid}")

    alloc_addr = kernel32.VirtualAllocEx(
        h_process, None, dll_path_len,
        MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE
    )
    if not alloc_addr:
        kernel32.CloseHandle(h_process)
        raise RuntimeError("VirtualAllocEx 失败")

    written = ctypes.c_size_t(0)
    if not kernel32.WriteProcessMemory(
        h_process, alloc_addr, dll_path_bytes, dll_path_len, ctypes.byref(written)
    ):
        kernel32.CloseHandle(h_process)
        raise RuntimeError("WriteProcessMemory 失败")

    kernel32_dll = ctypes.c_char_p(b"kernel32.dll")
    load_library_addr = kernel32.GetProcAddress(
        kernel32.GetModuleHandleW("kernel32.dll"), b"LoadLibraryA"
    )

    thread_id = ctypes.c_ulong(0)
    h_thread = kernel32.CreateRemoteThread(
        h_process, None, 0,
        load_library_addr, alloc_addr, 0, ctypes.byref(thread_id)
    )
    if not h_thread:
        kernel32.CloseHandle(h_process)
        raise RuntimeError("CreateRemoteThread 失败")

    kernel32.WaitForSingleObject(h_thread, 10000)
    kernel32.CloseHandle(h_thread)
    kernel32.CloseHandle(h_process)

    return True


def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    dll_path = os.path.join(script_dir, "hook.dll")

    if not os.path.exists(dll_path):
        print("=" * 50)
        print("  hook.dll 未找到，需要先编译")
        print("=" * 50)
        print()
        print("编译方法:")
        print("  1. 打开 Visual Studio Developer Command Prompt")
        print(f"  2. cd {script_dir}")
        print("  3. cl /LD hook.c /Fe:hook.dll")
        print()
        print("或者使用 MinGW:")
        print(f"  1. cd {script_dir}")
        print("  2. gcc -shared -o hook.dll hook.c")
        return

    print("[*] 查找微信进程...")
    pid = find_wechat_pid()
    print(f"[+] 找到 Weixin.exe PID={pid}")

    print(f"[*] 注入 {dll_path}...")
    inject_dll(pid, dll_path)
    print("[+] DLL 已注入")

    key_file = os.path.join(os.environ.get("TEMP", "."), "wechat_keys.txt")
    print(f"\n[*] 密钥将写入: {key_file}")
    print("[*] 请在微信中进行操作（如切换聊天）以触发数据库打开...")
    print("[*] 等待 10 秒...")

    time.sleep(10)

    if os.path.exists(key_file):
        print(f"\n[+] 找到密钥文件!")
        with open(key_file, "r", encoding="utf-8") as f:
            content = f.read()
        print(content)
    else:
        print(f"\n[!] 未生成密钥文件")
        print("可能原因:")
        print("  1. WeChat 使用 WCDB 内部 API 而非标准 sqlite3_key")
        print("  2. 微信没有在此期间打开新数据库")
        print("  3. 需要更精确的函数 Hook")


if __name__ == "__main__":
    main()
