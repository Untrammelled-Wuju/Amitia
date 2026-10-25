import assert from "node:assert/strict"
import test from "node:test"
import { authorizeServiceRequest } from "./service-auth.js"

const identity = {
  extensionId: "com.amitia/channel-qq-personal",
  moduleId: "qq-personal-service",
  token: "host-service-token",
}

const headers = {
  authorization: `Bearer ${identity.token}`,
  "x-amitia-extension-id": identity.extensionId,
  "x-amitia-module-id": identity.moduleId,
}

test("控制接口接受宿主签发的模块服务凭据", () => {
  assert.equal(authorizeServiceRequest(headers, identity), true)
})

test("控制接口拒绝缺失、错误或跨模块凭据", () => {
  assert.equal(authorizeServiceRequest({}, identity), false)
  assert.equal(authorizeServiceRequest({ ...headers, authorization: "Bearer wrong" }, identity), false)
  assert.equal(authorizeServiceRequest({ ...headers, "x-amitia-module-id": "other" }, identity), false)
  assert.equal(authorizeServiceRequest({ ...headers, "x-amitia-extension-id": "other" }, identity), false)
  assert.equal(authorizeServiceRequest(headers, { ...identity, token: "" }), false)
})
