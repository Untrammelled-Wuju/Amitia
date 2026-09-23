const int sidebarConversationPageSize = 5;

int expandSidebarConversationCount(int visibleCount, int total) {
  final maximum = total < sidebarConversationPageSize
      ? sidebarConversationPageSize
      : total;
  final next = visibleCount + sidebarConversationPageSize;
  return next > maximum ? maximum : next;
}

int normalizeSidebarConversationCount(int visibleCount, int total) {
  final maximum = total < sidebarConversationPageSize
      ? sidebarConversationPageSize
      : total;
  if (visibleCount < sidebarConversationPageSize) {
    return sidebarConversationPageSize;
  }
  return visibleCount > maximum ? maximum : visibleCount;
}
