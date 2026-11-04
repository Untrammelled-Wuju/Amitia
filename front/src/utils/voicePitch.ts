// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

export function normalizeVoicePitchRatio(value: unknown): number {
  const raw = Number(value);
  if (!Number.isFinite(raw) || raw === 0) return 1.0;
  return Number(Math.max(0.5, Math.min(2.0, raw)).toFixed(3));
}
