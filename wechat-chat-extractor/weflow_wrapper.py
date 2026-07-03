# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
import ctypes
from ctypes import wintypes
import subprocess
import time
import os
import json
import hashlib

WEFLOW_RES = r"E:\Code\Weflow\resources\resources"


class WeChatExtractor:
    def __init__(self):
        self.wx_key = None
        self.wcdb = None
        self.handle = None
        self._load_wx_key()
    
    def _load_wx_key(self):
        dll_path = os.path.join(WEFLOW_RES, "key", "win32", "x64", "wx_key.dll")
        if not os.path.exists(dll_path):
            raise FileNotFoundError(f"wx_key.dll not found: {dll_path}")
        self.wx_key = ctypes.CDLL(dll_path)
        
        self.wx_key.InitializeHook.argtypes = [ctypes.c_uint32]
        self.wx_key.InitializeHook.restype = ctypes.c_int
        self.wx_key.PollKeyData.argtypes = [ctypes.c_char_p, ctypes.c_int]
        self.wx_key.PollKeyData.restype = ctypes.c_int
        self.wx_key.GetImageKey.argtypes = [ctypes.c_char_p, ctypes.c_int]
        self.wx_key.GetImageKey.restype = ctypes.c_int
        self.wx_key.GetLastErrorMsg.restype = ctypes.c_char_p
        self.wx_key.GetStatusMessage.argtypes = [ctypes.c_char_p, ctypes.c_int, ctypes.POINTER(ctypes.c_int)]
        self.wx_key.GetStatusMessage.restype = ctypes.c_int
        self.wx_key.CleanupHook.restype = None
    
    def _find_wechat_pid(self):
        r = subprocess.run(
            ["tasklist", "/FI", "IMAGENAME eq Weixin.exe", "/FO", "CSV", "/NH"],
            capture_output=True, text=True
        )
        pids = []
        for line in r.stdout.strip().split("\n"):
            if not line.strip(): continue
            p = line.strip('"').split('","')
            if len(p) >= 5:
                pid = int(p[1])
                mem = int(p[4].replace(",", "").replace(" K", "").strip() or "0")
                pids.append((pid, mem))
        pids.sort(key=lambda x: x[1], reverse=True)
        return pids[0][0] if pids else None
    
    def get_image_key(self):
        buf = ctypes.create_string_buffer(4096)
        ret = self.wx_key.GetImageKey(buf, 4096)
        if ret:
            data = buf.value.decode("utf-8", errors="replace")
            return json.loads(data)
        return None
    
    def get_db_key(self, timeout_seconds=120):
        pid = self._find_wechat_pid()
        if not pid:
            raise RuntimeError("微信未运行")
        
        print(f"[+] 微信 PID: {pid}")
        
        ret = self.wx_key.InitializeHook(pid)
        if ret == 0:
            err = self.wx_key.GetLastErrorMsg()
            msg = err.decode("utf-8", errors="replace") if err else "未知错误"
            raise RuntimeError(f"InitializeHook 失败: {msg}")
        
        print("[+] Hook 已安装")
        print("[!] 请立即登出微信并重新登录！")
        print("[*] 等待登录过程...")
        
        key_buf = ctypes.create_string_buffer(256)
        start = time.time()
        last_status = ""
        
        while time.time() - start < timeout_seconds:
            if self.wx_key.PollKeyData(key_buf, 256):
                key = key_buf.value.decode("utf-8", errors="replace")
                if len(key) == 64 and all(c in "0123456789abcdef" for c in key):
                    print(f"\n[+] 密钥获取成功!")
                    print(f"    {key}")
                    self.wx_key.CleanupHook()
                    return key
            
            status_buf = ctypes.create_string_buffer(512)
            level = ctypes.c_int(0)
            if self.wx_key.GetStatusMessage(status_buf, 512, ctypes.byref(level)):
                msg = status_buf.value.decode("utf-8", errors="replace")
                if msg != last_status:
                    print(f"  [{level.value}] {msg}")
                    last_status = msg
            
            time.sleep(0.5)
        
        self.wx_key.CleanupHook()
        raise TimeoutError("超时未获取到密钥，请确保已登出并重新登录微信")
    
    def _load_wcdb(self):
        dll_path = os.path.join(WEFLOW_RES, "wcdb", "win32", "x64", "wcdb_api.dll")
        if not os.path.exists(dll_path):
            raise FileNotFoundError(f"wcdb_api.dll not found: {dll_path}")
        
        self.wcdb = ctypes.CDLL(dll_path)
        
        self.wcdb.wcdb_init.restype = ctypes.c_int32
        self.wcdb.wcdb_open_account.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.POINTER(ctypes.c_int64)]
        self.wcdb.wcdb_open_account.restype = ctypes.c_int32
        self.wcdb.wcdb_close_account.argtypes = [ctypes.c_int64]
        self.wcdb.wcdb_close_account.restype = ctypes.c_int32
        self.wcdb.wcdb_get_sessions.argtypes = [ctypes.c_int64, ctypes.POINTER(ctypes.c_void_p)]
        self.wcdb.wcdb_get_sessions.restype = ctypes.c_int32
        self.wcdb.wcdb_get_messages.argtypes = [ctypes.c_int64, ctypes.c_char_p, ctypes.c_int32, ctypes.c_int32, ctypes.POINTER(ctypes.c_void_p)]
        self.wcdb.wcdb_get_messages.restype = ctypes.c_int32
        self.wcdb.wcdb_get_message_count.argtypes = [ctypes.c_int64, ctypes.c_char_p, ctypes.POINTER(ctypes.c_int32)]
        self.wcdb.wcdb_get_message_count.restype = ctypes.c_int32
        self.wcdb.wcdb_get_display_names.argtypes = [ctypes.c_int64, ctypes.c_char_p, ctypes.POINTER(ctypes.c_void_p)]
        self.wcdb.wcdb_get_display_names.restype = ctypes.c_int32
        self.wcdb.wcdb_free_string.argtypes = [ctypes.c_void_p]
        self.wcdb.wcdb_free_string.restype = None
        self.wcdb.wcdb_shutdown.restype = ctypes.c_int32
        
        ret = self.wcdb.wcdb_init()
        if ret != 0:
            raise RuntimeError(f"WCDB 初始化失败: {ret}")
        print("[+] WCDB 已初始化")
    
    def open_account(self, db_key, wxid_path=None):
        if not self.wcdb:
            self._load_wcdb()
        
        key_bytes = db_key.encode("utf-8")
        path_bytes = wxid_path.encode("utf-8") if wxid_path else None
        
        handle = ctypes.c_int64(0)
        ret = self.wcdb.wcdb_open_account(
            ctypes.c_char_p(path_bytes) if path_bytes else None,
            key_bytes,
            ctypes.byref(handle)
        )
        if ret != 0:
            raise RuntimeError(f"打开账号失败: {ret}")
        
        self.handle = handle.value
        print(f"[+] 账号已打开")
        return self.handle
    
    def _read_json(self, ptr):
        if ptr:
            result = ctypes.cast(ptr, ctypes.c_char_p).value.decode("utf-8")
            self.wcdb.wcdb_free_string(ptr)
            return json.loads(result)
        return None
    
    def get_sessions(self):
        out_ptr = ctypes.c_void_p(0)
        ret = self.wcdb.wcdb_get_sessions(self.handle, ctypes.byref(out_ptr))
        if ret != 0:
            raise RuntimeError(f"获取会话失败: {ret}")
        return self._read_json(out_ptr.value) or []
    
    def get_messages(self, username, limit=50, offset=0):
        out_ptr = ctypes.c_void_p(0)
        ret = self.wcdb.wcdb_get_messages(
            self.handle, username.encode("utf-8"), limit, offset, ctypes.byref(out_ptr)
        )
        if ret != 0:
            raise RuntimeError(f"获取消息失败: {ret}")
        return self._read_json(out_ptr.value) or []
    
    def get_message_count(self, username):
        count = ctypes.c_int32(0)
        ret = self.wcdb.wcdb_get_message_count(
            self.handle, username.encode("utf-8"), ctypes.byref(count)
        )
        if ret != 0:
            raise RuntimeError(f"获取消息数失败: {ret}")
        return count.value
    
    def get_display_names(self, usernames):
        usernames_json = json.dumps(usernames)
        out_ptr = ctypes.c_void_p(0)
        ret = self.wcdb.wcdb_get_display_names(
            self.handle, usernames_json.encode("utf-8"), ctypes.byref(out_ptr)
        )
        if ret != 0:
            raise RuntimeError(f"获取昵称失败: {ret}")
        return self._read_json(out_ptr.value) or {}
    
    def close(self):
        if self.handle and self.wcdb:
            self.wcdb.wcdb_close_account(self.handle)
            self.wcdb.wcdb_shutdown()
            self.handle = None
            print("[+] 已关闭")


def get_wxid_path():
    base = r"D:\xwechat_files"
    if not os.path.isdir(base):
        return None
    for d in os.listdir(base):
        path = os.path.join(base, d)
        if os.path.isdir(path) and os.path.isdir(os.path.join(path, "db_storage")):
            return path
    return None


if __name__ == "__main__":
    extractor = WeChatExtractor()
    
    print("=" * 60)
    print("  WeChat 数据提取工具 (基于 WeFlow DLL)")
    print("=" * 60)
    
    print("\n[1] 获取图像密钥...")
    img_key = extractor.get_image_key()
    if img_key:
        print(f"  图像密钥: {json.dumps(img_key, indent=2, ensure_ascii=False)}")
    
    wxid_path = get_wxid_path()
    print(f"\n[2] 微信数据目录: {wxid_path}")
    
    print("\n[3] 获取数据库密钥...")
    print("    需要登出并重新登录微信。")
    print("    已自动开始扫描，请立即登出微信并重新登录...")
    
    try:
        db_key = extractor.get_db_key(timeout_seconds=180)
        
        print(f"\n[4] 打开账号并读取数据...")
        extractor.open_account(db_key, wxid_path)
        
        sessions = extractor.get_sessions()
        print(f"\n共 {len(sessions)} 个会话:")
        for s in sessions[:30]:
            tag = "群" if s.get("isGroup") else "私"
            name = s.get("displayName") or s.get("username", "?")
            count = s.get("messageCount", 0)
            print(f"  [{tag}] {name:<30} {count} 条")
        
        extractor.close()
        
    except Exception as e:
        print(f"\n[!] 错误: {e}")
