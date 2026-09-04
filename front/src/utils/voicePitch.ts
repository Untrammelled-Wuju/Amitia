// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Character voicePitch is persisted as a pitch ratio across every client and
 * TTS backend. Legacy Electron builds stored semitone values in the same
 * field; normalize obvious legacy values while keeping valid ratios intact.
 */
export function normalizeVoicePitchRatio(value: unknown): number {
  const raw = Number(value);
  if (!Number.isFinite(raw) || raw === 0) return 1.0;

  if (raw < 0 || raw > 2.0) {
    const semitones = Math.max(-12, Math.min(12, raw));
    return Number(Math.pow(2, semitones / 12).toFixed(3));
  }

  return Number(Math.max(0.5, Math.min(2.0, raw)).toFixed(3));
}
