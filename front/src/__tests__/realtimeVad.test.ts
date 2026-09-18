import { describe, expect, it } from "vitest";
import { RealtimeVoiceActivityDetector } from "../realtime/realtime-vad";

describe("RealtimeVoiceActivityDetector", () => {
  it("emits speech start and end around sustained voice activity", () => {
    const vad = new RealtimeVoiceActivityDetector({
      minRms: 0.01,
      noiseMultiplier: 2,
      speechStartFrames: 2,
      speechEndFrames: 2,
    });

    expect(vad.process(0.002)).toBeNull();
    expect(vad.process(0.04)).toBeNull();
    expect(vad.process(0.04)).toBe("speech_start");
    expect(vad.isSpeechActive).toBe(true);
    expect(vad.process(0.001)).toBeNull();
    expect(vad.process(0.001)).toBe("speech_end");
    expect(vad.isSpeechActive).toBe(false);
  });

  it("adapts its noise floor while idle", () => {
    const vad = new RealtimeVoiceActivityDetector({
      minRms: 0.005,
      noiseMultiplier: 3,
      speechStartFrames: 1,
      speechEndFrames: 1,
    });
    const initialThreshold = vad.threshold;

    for (let index = 0; index < 20; index++) {
      vad.process(0.008);
    }

    expect(vad.threshold).toBeGreaterThan(initialThreshold);
  });

  it("resets all state after an interrupted call", () => {
    const vad = new RealtimeVoiceActivityDetector({
      minRms: 0.01,
      speechStartFrames: 1,
      speechEndFrames: 1,
    });

    expect(vad.process(0.04)).toBe("speech_start");
    vad.reset();
    expect(vad.isSpeechActive).toBe(false);
    expect(vad.process(0.001)).toBeNull();
  });
});
