import type {
  FrameTimeline,
  LoopType,
  NormalizedFrame,
  TimelinePosition,
} from "./contracts";

export interface CumulativeBoundary {
  frameIndex: number;
  startMs: number;
  endMs: number;
}

export function buildCumulativeBoundaries(
  frames: readonly NormalizedFrame[],
): CumulativeBoundary[] {
  const boundaries: CumulativeBoundary[] = [];
  let cumulative = 0;
  for (const frame of frames) {
    boundaries.push({
      frameIndex: frame.index,
      startMs: cumulative,
      endMs: cumulative + frame.durationMs,
    });
    cumulative += frame.durationMs;
  }
  return boundaries;
}

function buildPingPongSequence(frameCount: number): number[] {
  if (frameCount <= 0) return [];
  if (frameCount === 1) return [0];
  if (frameCount === 2) return [0, 1];
  const seq: number[] = [];
  for (let i = 0; i < frameCount; i++) {
    seq.push(i);
  }
  for (let i = frameCount - 2; i >= 1; i--) {
    seq.push(i);
  }
  return seq;
}

function buildPingPongBoundaries(
  frames: readonly NormalizedFrame[],
  sequence: number[],
): CumulativeBoundary[] {
  const boundaries: CumulativeBoundary[] = [];
  let cumulative = 0;
  for (const frameIdx of sequence) {
    const frame = frames[frameIdx];
    if (!frame) continue;
    boundaries.push({
      frameIndex: frameIdx,
      startMs: cumulative,
      endMs: cumulative + frame.durationMs,
    });
    cumulative += frame.durationMs;
  }
  return boundaries;
}

function locateInBoundaries(
  boundaries: CumulativeBoundary[],
  localMs: number,
): { frameIndex: number; offsetMs: number } {
  if (boundaries.length === 0) {
    return { frameIndex: 0, offsetMs: 0 };
  }
  if (localMs <= 0) {
    return { frameIndex: boundaries[0].frameIndex, offsetMs: 0 };
  }
  let lo = 0;
  let hi = boundaries.length - 1;
  if (localMs >= boundaries[hi].endMs) {
    return {
      frameIndex: boundaries[hi].frameIndex,
      offsetMs: localMs - boundaries[hi].startMs,
    };
  }
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2);
    const b = boundaries[mid];
    if (localMs < b.startMs) {
      hi = mid - 1;
    } else if (localMs >= b.endMs) {
      lo = mid + 1;
    } else {
      return {
        frameIndex: b.frameIndex,
        offsetMs: localMs - b.startMs,
      };
    }
  }
  const b = boundaries[lo];
  return {
    frameIndex: b.frameIndex,
    offsetMs: Math.max(0, localMs - b.startMs),
  };
}

export function createFrameTimeline(
  frames: readonly NormalizedFrame[],
): FrameTimeline {
  const forwardBoundaries = buildCumulativeBoundaries(frames);
  const forwardDurationMs = forwardBoundaries.length > 0
    ? forwardBoundaries[forwardBoundaries.length - 1].endMs
    : 0;

  const pingPongSequence = buildPingPongSequence(frames.length);
  const pingPongBoundaries = buildPingPongBoundaries(frames, pingPongSequence);
  const pingPongDurationMs = pingPongBoundaries.length > 0
    ? pingPongBoundaries[pingPongBoundaries.length - 1].endMs
    : 0;

  const locate = (localMs: number, loopType: LoopType): TimelinePosition => {
    if (frames.length === 0) {
      return {
        frameIndex: 0,
        cycleIndex: 0,
        localMs: 0,
        completed: false,
      };
    }

    if (localMs < 0) {
      return {
        frameIndex: 0,
        cycleIndex: 0,
        localMs: 0,
        completed: false,
      };
    }

    if (frames.length === 1) {
      const singleFrameDuration = frames[0].durationMs;
      if (loopType === "once" || loopType === "hold") {
        const completed = localMs >= singleFrameDuration;
        return {
          frameIndex: 0,
          cycleIndex: 0,
          localMs,
          completed,
        };
      }
      return {
        frameIndex: 0,
        cycleIndex: 0,
        localMs,
        completed: false,
      };
    }

    switch (loopType) {
      case "loop": {
        if (forwardDurationMs <= 0) {
          return {
            frameIndex: 0,
            cycleIndex: 0,
            localMs,
            completed: false,
          };
        }
        const cycleIndex = Math.floor(localMs / forwardDurationMs);
        const cycleElapsed = localMs % forwardDurationMs;
        const found = locateInBoundaries(forwardBoundaries, cycleElapsed);
        return {
          frameIndex: found.frameIndex,
          cycleIndex,
          localMs,
          completed: false,
        };
      }

      case "once": {
        if (localMs >= forwardDurationMs) {
          const lastFrame = frames[frames.length - 1];
          return {
            frameIndex: lastFrame.index,
            cycleIndex: 0,
            localMs,
            completed: true,
          };
        }
        const found = locateInBoundaries(forwardBoundaries, localMs);
        return {
          frameIndex: found.frameIndex,
          cycleIndex: 0,
          localMs,
          completed: false,
        };
      }

      case "hold": {
        if (localMs >= forwardDurationMs) {
          const lastFrame = frames[frames.length - 1];
          return {
            frameIndex: lastFrame.index,
            cycleIndex: 0,
            localMs,
            completed: true,
          };
        }
        const found = locateInBoundaries(forwardBoundaries, localMs);
        return {
          frameIndex: found.frameIndex,
          cycleIndex: 0,
          localMs,
          completed: false,
        };
      }

      case "ping_pong": {
        if (pingPongDurationMs <= 0) {
          return {
            frameIndex: 0,
            cycleIndex: 0,
            localMs,
            completed: false,
          };
        }
        const cycleIndex = Math.floor(localMs / pingPongDurationMs);
        const cycleElapsed = localMs % pingPongDurationMs;
        const found = locateInBoundaries(pingPongBoundaries, cycleElapsed);
        return {
          frameIndex: found.frameIndex,
          cycleIndex,
          localMs,
          completed: false,
        };
      }

      default: {
        const found = locateInBoundaries(forwardBoundaries, localMs);
        return {
          frameIndex: found.frameIndex,
          cycleIndex: 0,
          localMs,
          completed: false,
        };
      }
    }
  };

  return {
    frames,
    forwardDurationMs,
    pingPongSequence,
    pingPongDurationMs,
    locate,
  };
}
