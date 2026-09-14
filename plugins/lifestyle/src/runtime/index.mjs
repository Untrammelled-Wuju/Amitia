const PROFILE_PREFIX = "v1/profile/";
const EVENTS_PREFIX = "v1/events/";

let activeHost = null;
let activeLogger = null;

const defaultTendencies = {
  punctualityTendency: 50,
  earlyPrepareTendency: 50,
  selfDisciplineTendency: 50,
  sleepinessTendency: 50,
  randomnessTendency: 50,
  activityEnergy: 50,
  socialEnergy: 50,
  careTendency: 50,
  dailyShareTendency: 50,
  manuallyConfigured: false,
};

function defaultProfile() {
  return {
    lifeIdentity: "CUSTOM",
    sleep: {
      bedTime: "23:00",
      wakeTime: "07:00",
      enabled: true,
      sleepReplyEnabled: false,
      sleepReplyMode: "NO_REPLY",
    },
    work: {
      enabled: false,
      workDays: "MON,TUE,WED,THU,FRI",
      workStartTime: "09:00",
      workEndTime: "18:00",
      lunchBreakStartTime: "12:00",
      lunchBreakEndTime: "13:30",
      commuteMinMinutes: 15,
      commuteMaxMinutes: 45,
      prepareMinMinutes: 20,
      prepareMaxMinutes: 60,
      replyMode: "SHORT_REPLY",
      allowOvertime: false,
      overtimeProbability: 10,
      overtimeMinMinutes: 30,
      overtimeMaxMinutes: 180,
      overtimeReplyMode: "SHORT_REPLY",
      delayedReplyEnabled: false,
      commuteHomeShareEnabled: true,
      commuteHomeShareProbability: 60,
    },
    tendencies: { ...defaultTendencies },
  };
}

function defaultEvents() {
  return {
    fixed: [],
    special: [],
    classAdjustments: [],
  };
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function number(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function boolean(value) {
  return value === true || value === 1 || value === "1";
}

function string(value, fallback = "") {
  return value === null || value === undefined ? fallback : String(value);
}

function dateKey(value) {
  const date = value instanceof Date ? value : new Date(value);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function parseDate(value) {
  const match = String(value || "").match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) return new Date();
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]), 0, 0, 0, 0);
}

function minutesOfDay(value, fallback = 0) {
  const match = String(value || "").match(/^(\d{1,2}):(\d{2})/);
  if (!match) return fallback;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) return fallback;
  return hour * 60 + minute;
}

function atMinutes(date, minutes) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate(), 0, minutes, 0, 0);
}

function addMinutes(date, minutes) {
  return new Date(date.getTime() + minutes * 60000);
}

function localISO(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  const second = String(date.getSeconds()).padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}:${second}`;
}

function normalizeProfile(value) {
  const base = defaultProfile();
  const input = value && typeof value === "object" ? value : {};
  return {
    lifeIdentity: string(input.lifeIdentity, base.lifeIdentity),
    sleep: {
      ...base.sleep,
      ...(input.sleep && typeof input.sleep === "object" ? input.sleep : {}),
    },
    work: {
      ...base.work,
      ...(input.work && typeof input.work === "object" ? input.work : {}),
    },
    tendencies: {
      ...base.tendencies,
      ...(input.tendencies && typeof input.tendencies === "object" ? input.tendencies : {}),
    },
  };
}

function normalizeEvents(value) {
  const input = value && typeof value === "object" ? value : {};
  return {
    fixed: Array.isArray(input.fixed) ? input.fixed : [],
    special: Array.isArray(input.special) ? input.special : [],
    classAdjustments: Array.isArray(input.classAdjustments) ? input.classAdjustments : [],
  };
}

async function readKey(host, key) {
  const response = await host.call("host.state.get", { key });
  return {
    value: response && response.found ? response.value : null,
    version: response && Number.isFinite(Number(response.version)) ? Number(response.version) : 0,
  };
}

async function writeKey(host, key, value) {
  const current = await readKey(host, key);
  const result = await host.call("host.state.cas", {
    key,
    expectedVersion: current.version,
    newValue: value,
  });
  if (!result || result.swapped !== true) {
    throw new Error(`state write conflict: ${key}`);
  }
  return Number(result.version || current.version + 1);
}

async function mutateKey(host, key, mutate) {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const current = await readKey(host, key);
    const next = mutate(current.value);
    const result = await host.call("host.state.cas", {
      key,
      expectedVersion: current.version,
      newValue: next,
    });
    if (result && result.swapped === true) return next;
  }
  throw new Error(`state mutation conflict: ${key}`);
}

function profileKey(characterId) {
  return `${PROFILE_PREFIX}${characterId}`;
}

function eventsKey(characterId) {
  return `${EVENTS_PREFIX}${characterId}`;
}

async function loadCharacter(host, characterId) {
  const profile = await readKey(host, profileKey(characterId));
  const events = await readKey(host, eventsKey(characterId));
  return {
    profile: normalizeProfile(profile.value),
    events: normalizeEvents(events.value),
    versions: {
      profile: profile.version,
      events: events.version,
    },
  };
}

function eventId(items, key) {
  return Number(key || 0) || Math.max(0, ...items.map((item) => number(item.id))) + 1;
}

function normalizeFixed(row) {
  return {
    id: number(row.id),
    title: string(row.title),
    description: string(row.description),
    weekDay: number(row.week_day, -1),
    startTime: string(row.start_time),
    endTime: string(row.end_time),
    eventType: string(row.event_type, "CUSTOM_BUSY"),
    repeatType: string(row.repeat_type, "weekly"),
    repeatDays: string(row.repeat_days),
    prepareMinMinutes: number(row.prepare_min_minutes, 10),
    prepareMaxMinutes: number(row.prepare_max_minutes, 40),
    replyMode: string(row.reply_mode, "SHORT_REPLY"),
    enabled: boolean(row.enabled),
  };
}

function normalizeSpecial(row) {
  return {
    id: number(row.id),
    title: string(row.title),
    description: string(row.description),
    startDate: string(row.start_date),
    endDate: string(row.end_date),
    startTime: string(row.start_time),
    endTime: string(row.end_time),
    eventType: string(row.event_type, "CUSTOM"),
    repeatType: string(row.repeat_type, "none"),
    repeatDays: string(row.repeat_days),
    enabled: boolean(row.enabled),
    priority: number(row.priority),
    activeMessageAllowed: boolean(row.active_message_allowed),
    replyMode: string(row.reply_mode, "SHORT_REPLY"),
    affectSchedule: boolean(row.affect_schedule),
    affectSleep: boolean(row.affect_sleep),
    affectMeal: boolean(row.affect_meal),
    affectEnergy: boolean(row.affect_energy),
  };
}

function normalizeAdjustment(row) {
  return {
    id: number(row.id),
    date: string(row.date),
    slotIndex: number(row.slot_index),
    className: string(row.class_name),
    adjustType: string(row.adjust_type, "swap"),
    description: string(row.description),
  };
}

function normalizeTendency(row) {
  return {
    ...defaultTendencies,
    punctualityTendency: number(row.punctuality_tendency, 50),
    earlyPrepareTendency: number(row.early_prepare_tendency, 50),
    selfDisciplineTendency: number(row.self_discipline_tendency, 50),
    sleepinessTendency: number(row.sleepiness_tendency, 50),
    randomnessTendency: number(row.randomness_tendency, 50),
    activityEnergy: number(row.activity_energy, 50),
    socialEnergy: number(row.social_energy, 50),
    careTendency: number(row.care_tendency, 50),
    dailyShareTendency: number(row.daily_share_tendency, 50),
    manuallyConfigured: boolean(row.manually_configured),
  };
}

function parseWorkDays(value) {
  const names = {
    SUN: 0,
    MON: 1,
    TUE: 2,
    WED: 3,
    THU: 4,
    FRI: 5,
    SAT: 6,
    "0": 0,
    "1": 1,
    "2": 2,
    "3": 3,
    "4": 4,
    "5": 5,
    "6": 6,
    "7": 0,
  };
  const result = new Set();
  for (const part of string(value).split(",").map((item) => item.trim().toUpperCase()).filter(Boolean)) {
    if (Object.prototype.hasOwnProperty.call(names, part)) {
      result.add(names[part]);
      continue;
    }
    const range = part.match(/^(\d|MON|TUE|WED|THU|FRI|SAT|SUN)-(\d|MON|TUE|WED|THU|FRI|SAT|SUN)$/);
    if (!range) continue;
    let start = names[range[1]];
    let end = names[range[2]];
    if (start > end) [start, end] = [end, start];
    for (let day = start; day <= end; day += 1) result.add(day);
  }
  return result;
}

function buildSchedule(profile, events, dateValue) {
  const date = parseDate(dateValue);
  const sleep = profile.sleep;
  let wakeMinutes = minutesOfDay(sleep.wakeTime, 420);
  let bedMinutes = minutesOfDay(sleep.bedTime, 1380);
  for (const event of events.fixed.filter((item) => item.enabled)) {
    if (event.eventType === "meal_lunch" && event.startTime) {
      profile.lunchMinutes = minutesOfDay(event.startTime, 720);
    }
  }
  const lunchMinutes = profile.lunchMinutes || 720;
  delete profile.lunchMinutes;
  let dinnerMinutes = 1110;
  let napStart = null;
  let napEnd = null;
  for (const event of events.fixed.filter((item) => item.enabled)) {
    if (event.eventType === "meal_dinner" && event.startTime) dinnerMinutes = minutesOfDay(event.startTime, dinnerMinutes);
    if (event.eventType === "nap" && event.startTime && event.endTime) {
      napStart = minutesOfDay(event.startTime);
      napEnd = minutesOfDay(event.endTime);
    }
  }
  if (number(profile.tendencies.activityEnergy, 50) < 30 && wakeMinutes < 420) wakeMinutes += 30;
  if (number(profile.tendencies.activityEnergy, 50) > 70 && wakeMinutes > 360) wakeMinutes -= 15;
  const isRestDay = events.special.some((event) => {
    if (!event.enabled || event.startDate !== dateKey(date)) return false;
    return event.eventType === "rest_day" || !event.startTime || (event.startTime === "00:00" && event.endTime === "23:59");
  });
  return {
    wakeTime: localISO(atMinutes(date, wakeMinutes)),
    lunchTime: localISO(atMinutes(date, lunchMinutes)),
    dinnerTime: localISO(atMinutes(date, dinnerMinutes)),
    hasNap: napStart !== null && napEnd !== null,
    napStartTime: napStart === null ? undefined : localISO(atMinutes(date, napStart)),
    napEndTime: napEnd === null ? undefined : localISO(atMinutes(date, napEnd)),
    sleepTime: localISO(atMinutes(date, bedMinutes)),
    isRestDay,
  };
}

function addEntry(entries, start, end, state, sourceType, priority, reason) {
  if (end <= start) return;
  entries.push({
    startTime: localISO(new Date(start)),
    endTime: localISO(new Date(end)),
    state,
    sourceType,
    priority,
    reason,
  });
}

function mergeEntries(entries) {
  const sorted = entries.map((item) => ({
    ...item,
    start: new Date(item.startTime).getTime(),
    end: new Date(item.endTime).getTime(),
  })).sort((left, right) => left.start - right.start);
  const result = [];
  for (const entry of sorted) {
    const last = result[result.length - 1];
    if (!last) {
      result.push(entry);
      continue;
    }
    if (entry.start < last.end) {
      if (entry.priority > last.priority) {
        last.end = entry.start;
        last.endTime = localISO(new Date(last.end));
        result.push(entry);
      }
      continue;
    }
    result.push(entry);
  }
  return result.map(({ start, end, ...entry }) => entry);
}

function buildTimeline(profile, events, dateValue) {
  const date = parseDate(dateValue);
  const schedule = buildSchedule(profile, events, dateValue);
  const wake = new Date(schedule.wakeTime);
  const lunch = new Date(schedule.lunchTime);
  const dinner = new Date(schedule.dinnerTime);
  const sleep = new Date(schedule.sleepTime);
  const dayStart = date.getTime();
  const nextDay = addMinutes(date, 1440);
  const entries = [];
  addEntry(entries, dayStart, wake, "SLEEPING", "schedule", 100, "睡眠时间");
  const wakeEnd = addMinutes(wake, 30);
  addEntry(entries, wake, wakeEnd, "WAKING_UP", "schedule", 90, "起床洗漱");
  const works = parseWorkDays(profile.work.workDays);
  const hasWork = profile.work.enabled && works.has(date.getDay());
  if (hasWork && !schedule.isRestDay) {
    const workStart = atMinutes(date, minutesOfDay(profile.work.workStartTime, 540));
    let workEnd = atMinutes(date, minutesOfDay(profile.work.workEndTime, 1080));
    if (workEnd <= workStart) workEnd = addMinutes(workEnd, 720);
    let cursor = wakeEnd;
    addEntry(entries, cursor, addMinutes(cursor, 30), "COMMUTING_TO_WORK", "work", 80, "上班通勤");
    cursor = addMinutes(cursor, 30);
    addEntry(entries, cursor, addMinutes(lunch, -30), "WORKING", "work", 75, "上午工作");
    addEntry(entries, addMinutes(lunch, -30), lunch, "LUNCH_BREAK", "schedule", 65, "休息");
    addEntry(entries, lunch, addMinutes(lunch, 60), "EATING_LUNCH", "schedule", 85, "午饭时间");
    cursor = addMinutes(lunch, 60);
    if (schedule.hasNap && schedule.napStartTime && schedule.napEndTime) {
      const napStart = new Date(schedule.napStartTime);
      const napEnd = new Date(schedule.napEndTime);
      addEntry(entries, cursor, napStart, "IDLE", "schedule", 40, "空闲");
      addEntry(entries, napStart, napEnd, "NAPPING", "schedule", 85, "午睡");
      cursor = napEnd;
    }
    addEntry(entries, cursor, workEnd, "WORKING", "work", 75, "下午工作");
    addEntry(entries, workEnd, addMinutes(workEnd, 30), "COMMUTING_HOME", "work", 80, "下班通勤");
    addEntry(entries, dinner, addMinutes(dinner, 60), "EATING_DINNER", "schedule", 85, "晚饭时间");
    addEntry(entries, addMinutes(dinner, 60), addMinutes(sleep, -60), "AFTER_WORK", "schedule", 50, "下班后自由时间");
  } else {
    const lunchEnd = addMinutes(lunch, 60);
    addEntry(entries, wakeEnd, lunch, schedule.isRestDay ? "IDLE" : "IDLE", "schedule", 50, schedule.isRestDay ? "休息日自由时间" : "自由时间");
    addEntry(entries, lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "午饭时间");
    if (schedule.hasNap && schedule.napStartTime && schedule.napEndTime) {
      addEntry(entries, lunchEnd, new Date(schedule.napStartTime), "IDLE", "schedule", 40, "空闲");
      addEntry(entries, new Date(schedule.napStartTime), new Date(schedule.napEndTime), "NAPPING", "schedule", 85, "午睡");
    }
    addEntry(entries, new Date(schedule.napEndTime || lunchEnd), dinner, "IDLE", "schedule", 45, "下午自由时间");
    addEntry(entries, dinner, addMinutes(dinner, 60), "EATING_DINNER", "schedule", 85, "晚饭时间");
    addEntry(entries, addMinutes(dinner, 60), addMinutes(sleep, -60), "IDLE", "schedule", 40, "晚间休息");
  }
  for (const adjustment of events.classAdjustments.filter((item) => item.date === dateKey(date) && item.adjustType !== "canceled")) {
    const start = atMinutes(date, 480 + number(adjustment.slotIndex) * 60);
    addEntry(entries, start, addMinutes(start, 50), "IN_CLASS", "class", 80, `课程: ${adjustment.className}`);
    addEntry(entries, addMinutes(start, 50), addMinutes(start, 65), "AFTER_CLASS", "class", 50, `课程结束: ${adjustment.className}`);
  }
  addEntry(entries, sleep, nextDay, "SLEEPING", "schedule", 100, "睡眠时间");
  return mergeEntries(entries);
}

function currentState(schedule, timeline, now) {
  const current = timeline.find((entry) => new Date(entry.startTime) <= now && now < new Date(entry.endTime));
  if (current) {
    const sleeping = current.state === "SLEEPING" || current.state === "NAPPING";
    const busy = ["IN_CLASS", "WORKING", "IN_EXAM", "BUSY", "OVERTIME"].includes(current.state);
    const available = ["IDLE", "AFTER_WORK", "AFTER_CLASS", "LIBRARY_BREAK", "LUNCH_BREAK"].includes(current.state);
    return {
      state: current.state,
      currentState: current.state,
      sleeping,
      busy,
      available,
      reason: current.reason,
      stateStartedAt: current.startTime,
      stateEndsAt: current.endTime,
    };
  }
  const wake = new Date(schedule.wakeTime);
  let sleep = new Date(schedule.sleepTime);
  if (sleep <= wake) sleep = addMinutes(sleep, 1440);
  const beforeSleep = addMinutes(sleep, -60);
  if (now < wake || now >= sleep) {
    return {
      state: "SLEEPING",
      currentState: "SLEEPING",
      sleeping: true,
      busy: false,
      available: false,
      reason: "睡眠时间",
      stateStartedAt: localISO(sleep),
      stateEndsAt: localISO(wake),
    };
  }
  if (now >= beforeSleep) {
    return {
      state: "BEFORE_SLEEP",
      currentState: "BEFORE_SLEEP",
      sleeping: false,
      busy: false,
      available: false,
      reason: "睡前准备",
      stateStartedAt: localISO(beforeSleep),
      stateEndsAt: localISO(sleep),
    };
  }
  return {
    state: "IDLE",
    currentState: "IDLE",
    sleeping: false,
    busy: false,
    available: true,
    reason: "空闲时间",
    stateStartedAt: localISO(wake),
    stateEndsAt: localISO(sleep),
  };
}

function stableNumber(value) {
  let hash = 2166136261;
  for (const character of String(value)) {
    hash ^= character.charCodeAt(0);
    hash = Math.imul(hash, 16777619);
  }
  return Math.abs(hash);
}

function calculateEnergy(now, schedule, state) {
  if (state === "SLEEPING" || state === "NAPPING") return 10 + stableNumber(dateKey(now)) % 15;
  if (["SICK_RESTING", "LOW_ENERGY", "LOW_ENERGY_AFTER_WORK"].includes(state)) return 10 + stableNumber(now.getHours()) % 31;
  const wake = new Date(schedule.wakeTime);
  const sleep = new Date(schedule.sleepTime);
  const lunch = new Date(schedule.lunchTime);
  const dinner = new Date(schedule.dinnerTime);
  if (now < addMinutes(wake, 60)) return 60 + stableNumber(now.getMinutes()) % 16;
  if (now < addMinutes(lunch, -60)) return 70 + stableNumber(now.getHours() * 60 + now.getMinutes()) % 21;
  if (now < lunch) return 50 + stableNumber(now.getMinutes()) % 26;
  if (now < addMinutes(dinner, -60)) return Math.max(40, 65 - Math.floor((now - lunch) / 3600000) * 3) + stableNumber(now.getMinutes()) % 16;
  if (now < addMinutes(dinner, 60)) return 55 + stableNumber(now.getMinutes()) % 16;
  if (now < addMinutes(sleep, -60)) return 50 + stableNumber(now.getMinutes()) % 16;
  return 30 + stableNumber(now.getMinutes()) % 16;
}

function conflicts(timeline) {
  const result = [];
  for (let i = 0; i < timeline.length; i += 1) {
    for (let j = i + 1; j < timeline.length; j += 1) {
      const left = timeline[i];
      const right = timeline[j];
      if (new Date(left.endTime) <= new Date(right.startTime) || new Date(right.endTime) <= new Date(left.startTime)) continue;
      result.push({
        type: "time_overlap",
        level: left.state === "SLEEPING" && ["IN_EXAM", "IN_CLASS"].includes(right.state) ? "error" : "warning",
        message: `${left.reason} 与 ${right.reason} 时间重叠`,
        startTime: left.startTime,
        endTime: left.endTime,
        sourceA: left.state,
        sourceB: right.state,
      });
    }
  }
  return result;
}

function realtimeContext(profile, events, now) {
  const fixed = events.fixed.filter((item) => item.enabled);
  for (const event of fixed) {
    if (event.weekDay >= 0 && event.weekDay !== now.getDay()) continue;
    const start = atMinutes(now, minutesOfDay(event.startTime, -1));
    const end = atMinutes(now, minutesOfDay(event.endTime, -1));
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end <= start) continue;
    if (now >= start && now < end) {
      return {
        context: `当前角色日程：正在${event.title}。回复应自然体现忙碌感，不要主动展开新话题。`,
        energyDelta: -0.02,
        stressDelta: 0.02,
      };
    }
    if (now < start && start - now <= 1800000) {
      return {
        context: `当前角色日程：即将开始${event.title}。回复可以简短自然，必要时说明稍后要去忙。`,
        energyDelta: -0.01,
        stressDelta: 0.01,
      };
    }
    if (now >= end && now - end <= 900000) {
      return {
        context: `当前角色日程：刚刚结束${event.title}。回复可以自然带一点放松感，不要主动播报计划。`,
        energyDelta: -0.01,
        stressDelta: 0,
      };
    }
  }
  if (profile.sleep.enabled) {
    const bedMinutes = minutesOfDay(profile.sleep.bedTime, 1380);
    const wakeMinutes = minutesOfDay(profile.sleep.wakeTime, 420);
    const nowMinutes = now.getHours() * 60 + now.getMinutes();
    const sleeping = bedMinutes > wakeMinutes
      ? nowMinutes >= bedMinutes || nowMinutes < wakeMinutes
      : nowMinutes >= bedMinutes && nowMinutes < wakeMinutes;
    if (sleeping) {
      return {
        context: "当前角色处于休息时段。回复应更轻、更短，不要主动拉长话题。",
        energyDelta: 0.01,
        stressDelta: -0.02,
      };
    }
  }
  return { context: "", energyDelta: 0, stressDelta: 0 };
}

function buildSnapshot(profile, events, at) {
  const now = at instanceof Date ? at : new Date(at || Date.now());
  const date = dateKey(now);
  const schedule = buildSchedule(profile, events, date);
  const timeline = buildTimeline(profile, events, date);
  const state = currentState(schedule, timeline, now);
  const energy = calculateEnergy(now, schedule, state.state);
  const stateLife = {
    currentState: state.currentState,
    currentActivity: state.reason,
    mood: "neutral",
    energy,
    idleDuration: 0,
    sleeping: state.sleeping,
    busy: state.busy,
    available: state.available,
    unifiedState: {
      busy: state.busy,
      replyable: state.available,
    },
    sleepSetting: profile.sleep,
    stateStartedAt: state.stateStartedAt,
    stateEndsAt: state.stateEndsAt,
  };
  return {
    state,
    stateLife,
    schedule,
    timeline: {
      date,
      events: timeline,
      schedule,
    },
    conflicts: conflicts(timeline),
    sleepSetting: profile.sleep,
    workProfile: profile.work,
    lifestyleTendency: profile.tendencies,
    fixedEvents: events.fixed,
    specialEvents: events.special,
    classAdjustments: events.classAdjustments,
    realtime: realtimeContext(profile, events, now),
  };
}

async function snapshot(host, characterId, at) {
  if (!characterId) throw new Error("缺少角色");
  const state = await loadCharacter(host, characterId);
  return buildSnapshot(state.profile, state.events, at);
}

async function updateProfile(host, characterId, payload) {
  const key = profileKey(characterId);
  return mutateKey(host, key, (current) => {
    const profile = normalizeProfile(current);
    if (payload.lifeIdentity !== undefined) profile.lifeIdentity = string(payload.lifeIdentity);
    if (payload.sleep && typeof payload.sleep === "object") profile.sleep = normalizeProfile({ ...profile, sleep: { ...profile.sleep, ...payload.sleep } }).sleep;
    if (payload.work && typeof payload.work === "object") profile.work = normalizeProfile({ ...profile, work: { ...profile.work, ...payload.work } }).work;
    if (payload.tendencies && typeof payload.tendencies === "object") profile.tendencies = normalizeProfile({ ...profile, tendencies: { ...profile.tendencies, ...payload.tendencies } }).tendencies;
    return profile;
  });
}

async function mutateEvents(host, characterId, mutate) {
  return mutateKey(host, eventsKey(characterId), (current) => {
    const events = normalizeEvents(current);
    return mutate(events);
  });
}

async function dispatchCommand(host, logger, input) {
  if (!host) throw new Error("生活系统尚未激活");
  const action = string(input && input.action);
  const payload = input && input.payload && typeof input.payload === "object" ? input.payload : {};
  const characterId = string(payload.characterId || input && input.characterId);
  switch (action) {
    case "snapshot":
      return snapshot(host, characterId, payload.at);
    case "profile.update":
      await updateProfile(host, characterId, payload);
      return snapshot(host, characterId, payload.at);
    case "sleep.update":
      await updateProfile(host, characterId, { sleep: payload.sleep || payload });
      return snapshot(host, characterId, payload.at);
    case "work.update":
      await updateProfile(host, characterId, { work: payload.work || payload });
      return snapshot(host, characterId, payload.at);
    case "tendency.update":
      await updateProfile(host, characterId, { tendencies: payload.tendencies || payload });
      return snapshot(host, characterId, payload.at);
    case "tendency.reset":
      await updateProfile(host, characterId, { tendencies: defaultTendencies });
      return snapshot(host, characterId, payload.at);
    case "fixed.create":
      await mutateEvents(host, characterId, (events) => {
        const item = normalizeFixed({ enabled: true, ...payload, id: eventId(events.fixed) });
        events.fixed.push(item);
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "fixed.update":
      await mutateEvents(host, characterId, (events) => {
        const index = events.fixed.findIndex((item) => item.id === number(payload.id));
        if (index < 0) throw new Error("固定事件不存在");
        events.fixed[index] = normalizeFixed({ ...events.fixed[index], ...payload });
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "fixed.toggle":
      await mutateEvents(host, characterId, (events) => {
        const item = events.fixed.find((entry) => entry.id === number(payload.id));
        if (!item) throw new Error("固定事件不存在");
        item.enabled = !item.enabled;
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "fixed.delete":
      await mutateEvents(host, characterId, (events) => {
        events.fixed = events.fixed.filter((item) => item.id !== number(payload.id));
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "special.create":
      await mutateEvents(host, characterId, (events) => {
        const item = normalizeSpecial({ enabled: true, activeMessageAllowed: true, ...payload, id: eventId(events.special) });
        events.special.push(item);
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "special.update":
      await mutateEvents(host, characterId, (events) => {
        const index = events.special.findIndex((item) => item.id === number(payload.id));
        if (index < 0) throw new Error("特殊事件不存在");
        events.special[index] = normalizeSpecial({ ...events.special[index], ...payload });
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "special.toggle":
      await mutateEvents(host, characterId, (events) => {
        const item = events.special.find((entry) => entry.id === number(payload.id));
        if (!item) throw new Error("特殊事件不存在");
        item.enabled = !item.enabled;
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "special.delete":
      await mutateEvents(host, characterId, (events) => {
        events.special = events.special.filter((item) => item.id !== number(payload.id));
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "class.create":
      await mutateEvents(host, characterId, (events) => {
        const item = normalizeAdjustment({ ...payload, id: eventId(events.classAdjustments) });
        events.classAdjustments.push(item);
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "class.update":
      await mutateEvents(host, characterId, (events) => {
        const index = events.classAdjustments.findIndex((item) => item.id === number(payload.id));
        if (index < 0) throw new Error("调课记录不存在");
        events.classAdjustments[index] = normalizeAdjustment({ ...events.classAdjustments[index], ...payload });
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "class.delete":
      await mutateEvents(host, characterId, (events) => {
        events.classAdjustments = events.classAdjustments.filter((item) => item.id !== number(payload.id));
        return events;
      });
      return snapshot(host, characterId, payload.at);
    case "schedule.regenerate":
    case "timeline.regenerate":
      return snapshot(host, characterId, payload.at);
    case "status":
      return { schedulerRunning: false };
    default:
      throw new Error(`未知生活系统操作: ${action}`);
  }
}

const lifestyleExtension = {
  async activate(context) {
    activeHost = context.host;
    activeLogger = context.log;
    context.handlers.bindTool("command", async (input) => dispatchCommand(activeHost, activeLogger, input));
    context.handlers.bindTool("context", async (input) => {
      const characterId = string(input && input.characterId);
      const at = input && input.at ? new Date(input.at) : new Date();
      return snapshot(activeHost, characterId, at);
    });
    context.log.info("lifestyle plugin activated");
  },
  async deactivate() {
    activeHost = null;
    activeLogger = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = lifestyleExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(lifestyleExtension);
}
