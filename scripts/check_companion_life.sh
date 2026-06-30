#!/bin/bash
# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
# U-Ai Companion Life 接口验收脚本
# 用法: bash scripts/check_companion_life.sh [BASE_URL]
BASE_URL="${1:-http://localhost:8080}"
PASS=0
FAIL=0

check() {
  local name="$1"
  local method="${2:-GET}"
  local path="$3"
  local url="${BASE_URL}${path}"
  local resp
  resp=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" 2>/dev/null)
  if [ "$resp" -ge 200 ] && [ "$resp" -lt 300 ]; then
    echo "[PASS] $resp $method $path"
    PASS=$((PASS + 1))
  else
    echo "[FAIL] $resp $method $path"
    FAIL=$((FAIL + 1))
  fi
}

check_body() {
  local name="$1"
  local method="${2:-GET}"
  local path="$3"
  local url="${BASE_URL}${path}"
  local resp
  resp=$(curl -s -w "\n%{http_code}" -X "$method" "$url" 2>/dev/null)
  local code
  code=$(echo "$resp" | tail -1)
  local body
  body=$(echo "$resp" | sed '$d')
  if [ "$code" -ge 200 ] && [ "$code" -lt 300 ] && [ -n "$body" ] && [ "$body" != "null" ] && [ "$body" != "{}" ]; then
    echo "[PASS] $code $method $path (body ok)"
    PASS=$((PASS + 1))
  else
    echo "[FAIL] $code $method $path (body empty/null)"
    FAIL=$((FAIL + 1))
  fi
}

echo "============================================"
echo "  U-Ai Companion Life API Check"
echo "  Base: $BASE_URL"
echo "============================================"

check      "今日作息"       GET  "/api/companion/schedule/today"
check      "状态时间轴"     GET  "/api/companion/timeline/today"
check      "当前状态"       GET  "/api/companion/state"
check      "生活状态"       GET  "/api/companion/state/life"
check_body "调试概览"       GET  "/api/companion/debug/overview"
check      "主动消息设置"   GET  "/api/companion/active-message/setting"
check_body "今日任务列表"   GET  "/api/companion/active-message/tasks/today"
check_body "作息重新生成"   POST "/api/companion/schedule/regenerate"
check_body "时间轴重新生成" POST "/api/companion/timeline/regenerate"
check      "触发日重生成"   POST "/api/companion/debug/trigger-daily-regeneration"
check      "处理主动消息"   POST "/api/companion/debug/process-active-messages"
check      "处理延迟回复"   POST "/api/companion/debug/process-delayed-replies"
check      "重新生成全部"   POST "/api/companion/debug/regenerate-all"

echo "============================================"
echo "  Results: $PASS passed, $FAIL failed"
echo "============================================"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1