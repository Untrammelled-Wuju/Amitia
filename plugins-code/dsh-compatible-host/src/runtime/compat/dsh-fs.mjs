export class FsError extends Error {
  constructor(message, code = "FS_ERROR", options) {
    super(message, options);
    this.name = "FsError";
    this.code = code;
  }
}
