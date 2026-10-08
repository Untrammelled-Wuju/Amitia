export const ROLE_AUTHORITY_HEADER = "X-Amitia-Role-Authority";

export function roleAuthorityConfig(authority: unknown) {
  if (typeof authority !== "string" || !/^[a-f0-9]{64}$/.test(authority)) {
    throw new Error("角色数据归属无法确认，请重新加载角色后再操作");
  }
  return { headers: { [ROLE_AUTHORITY_HEADER]: authority } };
}
