export const DEFAULT_LIST_LIMIT = 50;
export const MAX_LIST_LIMIT = 200;
export const MAX_TITLE_LENGTH = 200;
export const MIN_EVERY_INTERVAL_SECONDS = 60;

export function ScheduleId(value) {
  return value;
}

export class ScheduleInputError extends Error {
  constructor(message) {
    super(message);
    this.name = "ScheduleInputError";
    this.code = "SCHEDULE_INPUT_INVALID";
  }
}

export function formatScheduleLocalTime(value) {
  return String(value);
}
