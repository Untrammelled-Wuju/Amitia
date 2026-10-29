static long amitia_syscall0(long number) {
  register long x8 __asm__("x8") = number;
  register long x0 __asm__("x0");
  __asm__ volatile("svc #0" : "=r"(x0) : "r"(x8) : "memory");
  return x0;
}

static long amitia_syscall1(long number, long first) {
  register long x8 __asm__("x8") = number;
  register long x0 __asm__("x0") = first;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8) : "memory");
  return x0;
}

__attribute__((visibility("default"))) int setgid(unsigned int gid) {
  long current = amitia_syscall0(176);
  long result = amitia_syscall1(144, gid);
  if (result == -38 && current == gid) return 0;
  return result == 0 ? 0 : -1;
}

__attribute__((visibility("default"))) int setuid(unsigned int uid) {
  long current = amitia_syscall0(174);
  long result = amitia_syscall1(146, uid);
  if (result == -38 && current == uid) return 0;
  return result == 0 ? 0 : -1;
}
