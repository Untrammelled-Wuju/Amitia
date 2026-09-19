import fs from "node:fs";
import { EventEmitter } from "node:events";

class Entity {
  constructor(id, name, alias = "") {
    this.id = id;
    this.name = name;
    this.alias = alias;
  }

  async say(text) {
    fs.appendFileSync(process.env.AMITIA_WECHAT_TEST_OUTPUT, `${JSON.stringify({ to: this.id, text })}\n`);
  }
}

class TestBot extends EventEmitter {
  async start() {
    const self = new Entity("wxid_self", "测试微信", "amitia-test");
    const friend = new Entity("wxid_friend", "测试好友");
    setTimeout(() => this.emit("scan", "test-wechat-qr"), 10);
    setTimeout(() => this.emit("login", self), 20);
    setTimeout(() => this.emit("message", {
      self: () => false,
      text: () => "你好",
      talker: () => friend,
      room: () => null,
      id: () => "message-1",
    }), 35);
  }

  async logout() {}
  async stop() {}
}

export class PuppetWechat4u {
  constructor(options) {
    this.options = options;
  }
}

export const WechatyBuilder = {
  build: () => new TestBot(),
};
