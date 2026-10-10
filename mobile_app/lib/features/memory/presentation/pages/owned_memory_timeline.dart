DateTime? ownedMemoryResourceTime(Map<String, dynamic> row) {
  final body = row['body'];
  final content = body is Map ? body['content'] : null;
  for (final value in [
    row['updatedAt'],
    row['updated_at'],
    body is Map ? body['updatedAt'] : null,
    body is Map ? body['updated_at'] : null,
    content is Map ? content['updatedAt'] : null,
    content is Map ? content['updated_at'] : null,
    row['createdAt'],
    row['created_at'],
    body is Map ? body['createdAt'] : null,
    body is Map ? body['created_at'] : null,
    content is Map ? content['createdAt'] : null,
    content is Map ? content['created_at'] : null,
  ]) {
    if (value is String) {
      final parsed = DateTime.tryParse(value);
      if (parsed != null) return parsed;
    }
  }
  return null;
}

List<Map<String, dynamic>> ownedMemoryTimelineRows(
  Iterable<Map<String, dynamic>> rows,
) {
  final result = rows.toList();
  result.sort((a, b) {
    final left = ownedMemoryResourceTime(a);
    final right = ownedMemoryResourceTime(b);
    if (left != null && right != null) {
      final order = right.compareTo(left);
      if (order != 0) return order;
    } else if (left != null) {
      return -1;
    } else if (right != null) {
      return 1;
    }
    return '${a['ownerId']}/${a['id']}'.compareTo('${b['ownerId']}/${b['id']}');
  });
  return result;
}
