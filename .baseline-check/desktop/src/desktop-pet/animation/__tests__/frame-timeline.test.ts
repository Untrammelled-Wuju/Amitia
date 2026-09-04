import { describe, it, expect } from "vitest";
import type { NormalizedFrame, LoopType } from "../contracts";
import { createFrameTimeline, buildCumulativeBoundaries } from "../frame-timeline";

function makeFrames(count: number, durationMs: number = 100): NormalizedFrame[] {
  const frames: NormalizedFrame[] = [];
  let cumulative = 0;
  for (let i = 0; i < count; i++) {
    frames.push({
      index: i,
      resourceUrl: `frame_${i}.png`,
      durationMs,
      cumulativeStartMs: cumulative,
      cumulativeEndMs: cumulative + durationMs,
      frameId: `frame_${i}`,
      assetId: `asset_${i}`,
      contentHash: "0".repeat(64),
    });
    cumulative += durationMs;
  }
  return frames;
}

describe("buildCumulativeBoundaries", () => {
  it("with 3 frames", () => {
    const boundaries = buildCumulativeBoundaries(makeFrames(3, 100));
    expect(boundaries).toEqual([
      { frameIndex: 0, startMs: 0, endMs: 100 },
      { frameIndex: 1, startMs: 100, endMs: 200 },
      { frameIndex: 2, startMs: 200, endMs: 300 },
    ]);
  });

  it("with empty array", () => {
    expect(buildCumulativeBoundaries([])).toEqual([]);
  });
});

describe("createFrameTimeline", () => {
  it("forwardDurationMs calculation", () => {
    const frames = makeFrames(4, 100);
    const timeline = createFrameTimeline(frames);
    expect(timeline.forwardDurationMs).toBe(400);
    expect(timeline.frames.length).toBe(4);
  });

  it("pingPongSequence for 4 frames", () => {
    const timeline = createFrameTimeline(makeFrames(4, 100));
    expect(timeline.pingPongSequence).toEqual([0, 1, 2, 3, 2, 1]);
    expect(timeline.pingPongDurationMs).toBe(600);
  });

  it("pingPongSequence for 1 frame", () => {
    const timeline = createFrameTimeline(makeFrames(1, 100));
    expect(timeline.pingPongSequence).toEqual([0]);
    expect(timeline.pingPongDurationMs).toBe(100);
  });

  it("pingPongSequence for 2 frames", () => {
    const timeline = createFrameTimeline(makeFrames(2, 100));
    expect(timeline.pingPongSequence).toEqual([0, 1]);
    expect(timeline.pingPongDurationMs).toBe(200);
  });
});

describe("locate", () => {
  it("with loop type - start, middle, cycle boundary", () => {
    const timeline = createFrameTimeline(makeFrames(3, 100));
    const loop: LoopType = "loop";
    expect(timeline.locate(0, loop)).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 0,
      completed: false,
    });
    expect(timeline.locate(150, "loop")).toEqual({
      frameIndex: 1,
      cycleIndex: 0,
      localMs: 150,
      completed: false,
    });
    expect(timeline.locate(300, "loop")).toEqual({
      frameIndex: 0,
      cycleIndex: 1,
      localMs: 300,
      completed: false,
    });
    expect(timeline.locate(350, "loop")).toEqual({
      frameIndex: 0,
      cycleIndex: 1,
      localMs: 350,
      completed: false,
    });
  });

  it("with once type - before end, at end, after end", () => {
    const timeline = createFrameTimeline(makeFrames(3, 100));
    expect(timeline.locate(150, "once")).toEqual({
      frameIndex: 1,
      cycleIndex: 0,
      localMs: 150,
      completed: false,
    });
    expect(timeline.locate(300, "once")).toEqual({
      frameIndex: 2,
      cycleIndex: 0,
      localMs: 300,
      completed: true,
    });
    expect(timeline.locate(500, "once")).toEqual({
      frameIndex: 2,
      cycleIndex: 0,
      localMs: 500,
      completed: true,
    });
  });

  it("with hold type - before end, at end", () => {
    const timeline = createFrameTimeline(makeFrames(3, 100));
    expect(timeline.locate(150, "hold")).toEqual({
      frameIndex: 1,
      cycleIndex: 0,
      localMs: 150,
      completed: false,
    });
    expect(timeline.locate(300, "hold")).toEqual({
      frameIndex: 2,
      cycleIndex: 0,
      localMs: 300,
      completed: true,
    });
  });

  it("with ping_pong type - forward and backward phases", () => {
    const timeline = createFrameTimeline(makeFrames(4, 100));
    expect(timeline.locate(50, "ping_pong")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 50,
      completed: false,
    });
    expect(timeline.locate(350, "ping_pong")).toEqual({
      frameIndex: 3,
      cycleIndex: 0,
      localMs: 350,
      completed: false,
    });
    expect(timeline.locate(450, "ping_pong")).toEqual({
      frameIndex: 2,
      cycleIndex: 0,
      localMs: 450,
      completed: false,
    });
    expect(timeline.locate(550, "ping_pong")).toEqual({
      frameIndex: 1,
      cycleIndex: 0,
      localMs: 550,
      completed: false,
    });
    expect(timeline.locate(600, "ping_pong")).toEqual({
      frameIndex: 0,
      cycleIndex: 1,
      localMs: 600,
      completed: false,
    });
    expect(timeline.locate(650, "ping_pong")).toEqual({
      frameIndex: 0,
      cycleIndex: 1,
      localMs: 650,
      completed: false,
    });
  });

  it("with empty frames", () => {
    const timeline = createFrameTimeline([]);
    expect(timeline.forwardDurationMs).toBe(0);
    expect(timeline.pingPongSequence).toEqual([]);
    expect(timeline.pingPongDurationMs).toBe(0);
    expect(timeline.locate(100, "loop")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 0,
      completed: false,
    });
  });

  it("with negative localMs", () => {
    const timeline = createFrameTimeline(makeFrames(3, 100));
    expect(timeline.locate(-50, "loop")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 0,
      completed: false,
    });
    expect(timeline.locate(-1, "once")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 0,
      completed: false,
    });
    expect(timeline.locate(-100, "ping_pong")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 0,
      completed: false,
    });
  });

  it("with single frame", () => {
    const timeline = createFrameTimeline(makeFrames(1, 100));
    expect(timeline.locate(50, "loop")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 50,
      completed: false,
    });
    expect(timeline.locate(250, "loop")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 250,
      completed: false,
    });
    expect(timeline.locate(50, "once")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 50,
      completed: false,
    });
    expect(timeline.locate(100, "once")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 100,
      completed: true,
    });
    expect(timeline.locate(200, "once")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 200,
      completed: true,
    });
    expect(timeline.locate(100, "hold")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 100,
      completed: true,
    });
    expect(timeline.locate(250, "ping_pong")).toEqual({
      frameIndex: 0,
      cycleIndex: 0,
      localMs: 250,
      completed: false,
    });
  });
});
