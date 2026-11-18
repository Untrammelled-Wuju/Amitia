function normalizeDelay(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return 0;
  return Math.max(0, parsed);
}

export function createDshTimerService(options = {}) {
  const setTimer = typeof options.setTimeout === "function" ? options.setTimeout : (handler, delay) => setTimeout(handler, delay);
  const clearTimer = typeof options.clearTimeout === "function" ? options.clearTimeout : (handle) => clearTimeout(handle);
  const setRepeater = typeof options.setInterval === "function" ? options.setInterval : (handler, delay) => setInterval(handler, delay);
  const clearRepeater = typeof options.clearInterval === "function" ? options.clearInterval : (handle) => clearInterval(handle);
  const timers = new Map();
  let disposed = false;

  function assertActive() {
    if (disposed) throw new Error("DSH timer service is disposed");
  }

  function schedule(handler, delay) {
    const handle = setTimer(() => {
      timers.delete(handle);
      handler();
    }, delay);
    timers.set(handle, clearTimer);
    return handle;
  }

  function cancel(handle) {
    const clear = timers.get(handle);
    if (!clear) return;
    timers.delete(handle);
    clear(handle);
  }

  function timeout(callback, delay) {
    assertActive();
    if (typeof callback === "function") {
      const handle = schedule(() => callback(), normalizeDelay(delay));
      return () => cancel(handle);
    }
    return new Promise((resolve) => {
      schedule(() => resolve(), normalizeDelay(callback));
    });
  }

  function interval(callback, delay) {
    assertActive();
    if (typeof callback === "function") {
      const handle = setRepeater(callback, normalizeDelay(delay));
      timers.set(handle, clearRepeater);
      return () => cancel(handle);
    }
    const wait = normalizeDelay(callback);
    const queue = [];
    let resolveNext = null;
    let stopped = false;
    const handle = setRepeater(() => {
      if (resolveNext) {
        const resume = resolveNext;
        resolveNext = null;
        resume();
        return;
      }
      queue.push(undefined);
    }, wait);
    timers.set(handle, clearRepeater);
    const stop = () => {
      if (stopped) return;
      stopped = true;
      cancel(handle);
      const resume = resolveNext;
      resolveNext = null;
      resume?.();
    };
    return {
      next() {
        if (stopped) return Promise.resolve({ done: true, value: undefined });
        if (queue.length > 0) {
          queue.shift();
          return Promise.resolve({ done: false, value: undefined });
        }
        return new Promise((resolve) => {
          resolveNext = () => {
            if (!stopped && queue.length > 0) queue.shift();
            resolve({ done: stopped, value: undefined });
          };
        });
      },
      return() {
        stop();
        return Promise.resolve({ done: true, value: undefined });
      },
      [Symbol.asyncIterator]() {
        return this;
      },
      dispose: stop,
    };
  }

  function throttle(callback, delay, noTrailing) {
    assertActive();
    if (typeof callback !== "function") throw new Error("DSH timer.throttle callback must be a function");
    const wait = normalizeDelay(delay);
    let lastInvokedAt = Number.NEGATIVE_INFINITY;
    let trailingHandle = null;
    let trailingArgs = null;
    let trailingContext = null;
    let active = true;
    const invoke = (context, args) => {
      lastInvokedAt = Date.now();
      return callback.apply(context, args);
    };
    const cancelTrailing = () => {
      if (trailingHandle === null) return;
      cancel(trailingHandle);
      trailingHandle = null;
      trailingArgs = null;
      trailingContext = null;
    };
    const throttled = function throttled(...args) {
      if (disposed) throw new Error("DSH timer service is disposed");
      if (!active) return undefined;
      const now = Date.now();
      if (now - lastInvokedAt >= wait) {
        cancelTrailing();
        return invoke(this, args);
      }
      if (noTrailing === true) return undefined;
      trailingArgs = args;
      trailingContext = this;
      if (trailingHandle === null) {
        const remaining = Math.max(0, wait - (now - lastInvokedAt));
        trailingHandle = schedule(() => {
          trailingHandle = null;
          const pendingArgs = trailingArgs;
          const pendingContext = trailingContext;
          trailingArgs = null;
          trailingContext = null;
          if (pendingArgs) invoke(pendingContext, pendingArgs);
        }, remaining);
      }
      return undefined;
    };
    throttled.dispose = () => {
      active = false;
      cancelTrailing();
    };
    return throttled;
  }

  function debounce(callback, delay) {
    assertActive();
    if (typeof callback !== "function") throw new Error("DSH timer.debounce callback must be a function");
    const wait = normalizeDelay(delay);
    let handle = null;
    let active = true;
    const cancelPending = () => {
      if (handle === null) return;
      cancel(handle);
      handle = null;
    };
    const debounced = function debounced(...args) {
      if (disposed) throw new Error("DSH timer service is disposed");
      if (!active) return undefined;
      const context = this;
      if (handle !== null) cancel(handle);
      handle = schedule(() => {
        handle = null;
        callback.apply(context, args);
      }, wait);
      return undefined;
    };
    debounced.dispose = () => {
      active = false;
      cancelPending();
    };
    return debounced;
  }

  return Object.freeze({
    timeout,
    interval,
    throttle,
    debounce,
    dispose() {
      if (disposed) return;
      disposed = true;
      for (const handle of [...timers.keys()]) {
        const clear = timers.get(handle);
        timers.delete(handle);
        clear?.(handle);
      }
    },
  });
}
