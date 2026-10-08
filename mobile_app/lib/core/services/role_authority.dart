Map<String, String>? roleAuthorityHeaders(String? authority) {
  if (authority == null) return null;
  if (!RegExp(r'^[a-f0-9]{64}$').hasMatch(authority)) {
    throw StateError('角色数据归属无法确认，请重新加载角色后再操作');
  }
  return {'X-Amitia-Role-Authority': authority};
}
