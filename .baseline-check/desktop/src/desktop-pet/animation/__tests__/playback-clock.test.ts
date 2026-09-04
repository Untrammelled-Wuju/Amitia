import { describe, it, expect } from "vitest";
import { MonotonicPlaybackClock, FakePlaybackClock } from "../playback-clock";

describe("MonotonicPlaybackClock", () => {
  it("now returns increasing values", async () => {
    const clock = new MonotonicPlaybackClock();
    const first = clock.now();
    expect(typeof first).toBe("number");
    await new Promise((resolve) => setTimeout(resolve, 10));
    const second = clock.now();
    await new Promise((resolve) => setTimeout(resolve, 10));
    const third = clock.now();
    expect(second).toBeGreaterThanOrEqual(first);
    expect(third).toBeGreaterThanOrEqual(second);
    expect(third).toBeGreaterThan(first);
  });

  it("requestTick fires callback", async () => {
    const clock = new MonotonicPlaybackClock();
    let received: number | null = null;
    const handle = clock.requestTick((ts) => {
      received = ts;
    });
    expect(typeof handle).toBe("number");
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(received).not.toBeNull();
    expect(typeof received).toBe("number");
  });

  it("cancelTick prevents callback", async () => {
    const clock = new MonotonicPlaybackClock();
    let called = false;
    const handle = clock.requestTick(() => {
      called = true;
    });
    clock.cancelTick(handle);
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(called).toBe(false);
  });

  it("cancelAll clears all", async () => {
    const clock = new MonotonicPlaybackClock();
    let calls = 0;
    clock.requestTick(() => {
      calls += 1;
    });
    clock.requestTick(() => {
      calls += 1;
    });
    clock.requestTick(() => {
      calls += 1;
    });
    clock.cancelAll();
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(calls).toBe(0);
  });
});

describe("FakePlaybackClock", () => {
  it("starts at 0", () => {
    const clock = new FakePlaybackClock();
    expect(clock.now()).toBe(0);
    expect(clock.getPendingCount()).toBe(0);
  });

  it("advance moves time and fires ticks", () => {
    const clock = new FakePlaybackClock();
    const calls: number[] = [];
    clock.requestTick((now) => {
      calls.push(now);
    });
    expect(clock.getPendingCount()).toBe(1);
    clock.advance(50);
    expect(clock.now()).toBe(50);
    expect(calls).toEqual([50]);
    expect(clock.getPendingCount()).toBe(0);
  });

  it("advanceTo only moves forward", () => {
    const clock = new FakePlaybackClock();
    clock.advanceTo(100);
    expect(clock.now()).toBe(100);
    clock.advanceTo(50);
    expect(clock.now()).toBe(100);
    clock.advanceTo(100);
    expect(clock.now()).toBe(100);
    clock.advanceTo(200);
    expect(clock.now()).toBe(200);
  });

  it("reset clears everything", () => {
    const clock = new FakePlaybackClock();
    clock.requestTick(() => {});
    clock.advance(100);
    clock.requestTick(() => {});
    expect(clock.getPendingCount()).toBe(1);
    clock.reset();
    expect(clock.now()).toBe(0);
    expect(clock.getPendingCount()).toBe(0);
  });

  it("cancelTick removes specific tick", () => {
    const clock = new FakePlaybackClock();
    const fired: number[] = [];
    const h1 = clock.requestTick(() => {
      fired.push(1);
    });
    const h2 = clock.requestTick(() => {
      fired.push(2);
    });
    const h3 = clock.requestTick(() => {
      fired.push(3);
    });
    expect(h1).not.toBe(h2);
    expect(h2).not.toBe(h3);
    clock.cancelTick(h2);
    expect(clock.getPendingCount()).toBe(2);
    clock.advance(10);
    expect(fired).toEqual([1, 3]);
  });

  it("getPendingCount reflects state", () => {
    const clock = new FakePlaybackClock();
    expect(clock.getPendingCount()).toBe(0);
    const h1 = clock.requestTick(() => {});
    expect(clock.getPendingCount()).toBe(1);
    clock.requestTick(() => {});
    expect(clock.getPendingCount()).toBe(2);
    clock.cancelTick(h1);
    expect(clock.getPendingCount()).toBe(1);
    clock.advance(5);
    expect(clock.getPendingCount()).toBe(0);
  });
});
