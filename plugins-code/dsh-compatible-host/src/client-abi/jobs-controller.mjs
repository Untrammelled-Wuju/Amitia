function text(value) {
  return value == null ? "" : String(value).trim();
}

function orderedJobs(items) {
  return [...items].sort((left, right) => {
    const liveLeft = left.status === "running" || left.status === "stopping";
    const liveRight = right.status === "running" || right.status === "stopping";
    if (liveLeft !== liveRight) return liveLeft ? -1 : 1;
    return (right.finishedAt ?? right.startedAt) - (left.finishedAt ?? left.startedAt) || right.startedAt - left.startedAt;
  });
}

export function createDshClientJobsController(command, options = {}) {
  if (typeof command !== "function") throw new Error("DSH jobs host command is unavailable");
  const pollInterval = Number.isFinite(Number(options.pollInterval)) ? Math.max(100, Number(options.pollInterval)) : 750;
  const maxObservationBytes = Number.isFinite(Number(options.maxObservationBytes)) ? Math.max(1024, Number(options.maxObservationBytes)) : 64 * 1024;
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();
  const rowsWatched = new Map();
  const observed = new Map();
  const listeners = new Set();
  let snapshot = Object.freeze({ rows: Object.freeze({}), observed: Object.freeze({}) });
  let timer = null;
  let refreshing = null;
  let disposed = false;

  const changed = () => {
    snapshot = Object.freeze({
      rows: Object.freeze(snapshot.rows),
      observed: Object.freeze(snapshot.observed),
    });
    for (const listener of Array.from(listeners)) listener();
  };

  const refresh = () => {
    if (disposed) return Promise.resolve(snapshot);
    if (refreshing) return refreshing;
    refreshing = Promise.resolve(command({ action: "jobs-list" })).then((result) => {
      if (disposed) return snapshot;
      const rows = {};
      const observedState = {};
      for (const job of result?.jobs ?? []) {
        const sessionId = text(job.owner);
        if (!sessionId || !rowsWatched.has(sessionId)) continue;
        const list = rows[sessionId] ?? [];
        list.push(job);
        rows[sessionId] = list;
        if (observed.has(job.id)) {
          const encoded = encoder.encode(String(job.text ?? ""));
          const bounded = encoded.length > maxObservationBytes ? encoded.slice(encoded.length - maxObservationBytes) : encoded;
          observedState[job.id] = Object.freeze({
            jobId: job.id,
            text: decoder.decode(bounded),
            gapBefore: encoded.length > maxObservationBytes,
            streaming: job.status === "running" || job.status === "stopping",
          });
        }
      }
      for (const sessionId of Object.keys(rows)) rows[sessionId] = Object.freeze(orderedJobs(rows[sessionId]));
      snapshot = Object.freeze({ rows: Object.freeze(rows), observed: Object.freeze(observedState) });
      changed();
      return snapshot;
    }).catch((error) => {
      console.error("[original-error]", error);
      throw error;
    }).finally(() => {
      refreshing = null;
    });
    return refreshing;
  };

  const ensureTimer = () => {
    if (timer !== null || disposed || rowsWatched.size === 0) return;
    timer = setInterval(() => {
      void refresh().catch((error) => console.error("[original-error]", error));
    }, pollInterval);
  };

  const stopTimerIfIdle = () => {
    if (timer === null || rowsWatched.size > 0) return;
    clearInterval(timer);
    timer = null;
  };

  return {
    state: {
      getSnapshot() {
        return snapshot;
      },
      subscribe(listener) {
        if (typeof listener !== "function") throw new Error("DSH jobs subscriber must be a function");
        listeners.add(listener);
        return () => listeners.delete(listener);
      },
    },
    watchRows(sessionId) {
      const id = text(sessionId);
      if (!id) throw new Error("DSH jobs watchRows sessionId is required");
      rowsWatched.set(id, (rowsWatched.get(id) ?? 0) + 1);
      ensureTimer();
      void refresh().catch((error) => console.error("[original-error]", error));
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const count = (rowsWatched.get(id) ?? 1) - 1;
        if (count > 0) rowsWatched.set(id, count);
        else rowsWatched.delete(id);
        if (!rowsWatched.has(id) && snapshot.rows[id] !== undefined) {
          const rows = { ...snapshot.rows };
          delete rows[id];
          snapshot = Object.freeze({ rows: Object.freeze(rows), observed: snapshot.observed });
          changed();
        }
        stopTimerIfIdle();
      };
    },
    observe(_sessionId, jobId) {
      const id = text(jobId);
      if (!id) throw new Error("DSH jobs observe jobId is required");
      observed.set(id, (observed.get(id) ?? 0) + 1);
      void refresh().catch((error) => console.error("[original-error]", error));
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const count = (observed.get(id) ?? 1) - 1;
        if (count > 0) {
          observed.set(id, count);
          return;
        }
        observed.delete(id);
        if (snapshot.observed[id] === undefined) return;
        const next = { ...snapshot.observed };
        delete next[id];
        snapshot = Object.freeze({ rows: snapshot.rows, observed: Object.freeze(next) });
        changed();
      };
    },
    async kill(sessionId, jobId) {
      const owner = text(sessionId);
      const id = text(jobId);
      if (!id) return { ok: false, error: "jobId is required" };
      const job = (snapshot.rows[owner] ?? []).find((item) => item.id === id);
      try {
        await command({ action: "jobs-kill", pluginId: job?.pluginId, jobId: id, reason: "用户停止" });
        await refresh();
        return { ok: true };
      } catch (error) {
        return { ok: false, error };
      }
    },
    async dispose() {
      if (disposed) return;
      disposed = true;
      if (timer !== null) clearInterval(timer);
      timer = null;
      rowsWatched.clear();
      observed.clear();
      listeners.clear();
      snapshot = Object.freeze({ rows: Object.freeze({}), observed: Object.freeze({}) });
    },
  };
}
