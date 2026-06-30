// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
#include <windows.h>
#include <stdio.h>
#include <psapi.h>

typedef int (*sqlite3_key_type)(void*, const void*, int);

static FILE* g_log = NULL;

void write_key(const char* db_name, const unsigned char* key, int key_len) {
    if (!g_log) {
        char path[MAX_PATH];
        GetEnvironmentVariableA("TEMP", path, MAX_PATH);
        int plen = (int)strlen(path);
        if (plen > 0 && path[plen-1] != 0x5c) { path[plen]=0x5c; path[plen+1]=0; }
        strcat(path, "wechat_keys.txt");
        g_log = fopen(path, "a");
    }
    if (g_log && key && key_len > 0) {
        fprintf(g_log, "DB: %s\nKey:", db_name ? db_name : "null");
        int i;
        for (i = 0; i < key_len; i++) fprintf(g_log, "%02x", key[i]);
        fprintf(g_log, "\n---\n");
        fflush(g_log);
    }
}

typedef int (*p_sqlite3_key)(void*, const void*, int);

static p_sqlite3_key g_real_sqlite3_key = NULL;

int Hooked_sqlite3_key(void* db, const void* key, int key_len) {
    if (key && key_len > 0 && key_len <= 256) {
        write_key("sqlite3_key", (const unsigned char*)key, key_len);
    }
    if (g_real_sqlite3_key) {
        return g_real_sqlite3_key(db, key, key_len);
    }
    return 0;
}

static void try_hook_module(HMODULE hMod) {
    void* func = (void*)GetProcAddress(hMod, "sqlite3_key");
    if (func) {
        g_real_sqlite3_key = (p_sqlite3_key)func;
        char dbg[256];
        _snprintf(dbg, sizeof(dbg), "Found sqlite3_key at %p", func);
        OutputDebugStringA(dbg);
    }
}

__declspec(dllexport) void Hook(void) {
    OutputDebugStringA("[WeChatHook] Hook() called, searching for sqlite3_key...");
    try_hook_module(GetModuleHandleA(NULL));
    
    HMODULE hMods[1024];
    DWORD needed;
    HANDLE hProc = GetCurrentProcess();
    if (EnumProcessModules(hProc, hMods, sizeof(hMods), &needed)) {
        int count = needed / sizeof(HMODULE);
        int i;
        for (i = 0; i < count; i++) {
            char name[MAX_PATH];
            if (GetModuleFileNameExA(hProc, hMods[i], name, MAX_PATH)) {
                char* slash = strrchr(name, 0x5c);
                char* base = slash ? slash + 1 : name;
                if (_stricmp(base, "Weixin.dll") == 0) {
                    try_hook_module(hMods[i]);
                }
            }
        }
    }
    
    if (g_real_sqlite3_key) {
        OutputDebugStringA("[WeChatHook] Found sqlite3_key - ready to capture");
    } else {
        OutputDebugStringA("[WeChatHook] sqlite3_key NOT found in any module");
    }
}

BOOL APIENTRY DllMain(HMODULE hModule, DWORD reason, LPVOID reserved) {
    (void)hModule; (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(hModule);
        Hook();
    }
    if (reason == DLL_PROCESS_DETACH && g_log) {
        fclose(g_log);
        g_log = NULL;
    }
    return TRUE;
}
