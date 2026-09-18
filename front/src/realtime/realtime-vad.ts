export type RealtimeVadEvent = "speech_start" | "speech_end" | null;

export interface RealtimeVadOptions {
  minRms: number;
  noiseMultiplier: number;
  speechStartFrames: number;
  speechEndFrames: number;
}

const defaultOptions: RealtimeVadOptions = {
  minRms: 0.018,
  noiseMultiplier: 3.2,
  speechStartFrames: 2,
  speechEndFrames: 2,
};

export class RealtimeVoiceActivityDetector {
  private readonly options: RealtimeVadOptions;
  private noiseFloor = 0.004;
  private active = false;
  private speechFrames = 0;
  private silenceFrames = 0;

  constructor(options: Partial<RealtimeVadOptions> = {}) {
    this.options = { ...defaultOptions, ...options };
  }

  get isSpeechActive(): boolean {
    return this.active;
  }

  get threshold(): number {
    return Math.max(
      this.options.minRms,
      Math.min(0.12, this.noiseFloor * this.options.noiseMultiplier),
    );
  }

  reset(): void {
    this.noiseFloor = 0.004;
    this.active = false;
    this.speechFrames = 0;
    this.silenceFrames = 0;
  }

  process(rms: number): RealtimeVadEvent {
    if (!Number.isFinite(rms) || rms < 0) return null;
    const speech = rms >= this.threshold;

    if (!this.active) {
      if (speech) {
        this.speechFrames++;
        this.silenceFrames = 0;
      } else {
        this.speechFrames = 0;
        this.noiseFloor = this.noiseFloor * 0.95 + Math.min(rms, 0.05) * 0.05;
      }
      if (this.speechFrames >= this.options.speechStartFrames) {
        this.active = true;
        this.speechFrames = 0;
        this.silenceFrames = 0;
        return "speech_start";
      }
      return null;
    }

    if (speech) {
      this.silenceFrames = 0;
      return null;
    }

    this.silenceFrames++;
    this.noiseFloor = this.noiseFloor * 0.98 + Math.min(rms, 0.05) * 0.02;
    if (this.silenceFrames >= this.options.speechEndFrames) {
      this.active = false;
      this.silenceFrames = 0;
      return "speech_end";
    }
    return null;
  }
}
