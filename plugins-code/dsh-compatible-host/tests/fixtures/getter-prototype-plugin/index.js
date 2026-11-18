export default class AmitiaTestGetterPrototypeService {
  constructor() {
    globalThis.__dshGetterProbe = { constructed: true };
  }

  get view() {
    throw new TypeError("prototype getter must not be read during plugin loading");
  }

  method() {
    return true;
  }
}
