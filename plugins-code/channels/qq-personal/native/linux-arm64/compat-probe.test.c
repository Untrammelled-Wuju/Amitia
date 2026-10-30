#define _GNU_SOURCE
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/prctl.h>
#include <sys/uio.h>
#include <unistd.h>

static void *check_thread_name(void *unused) {
  (void)unused;
  char name[16] = {0};
  prctl(PR_SET_NAME, "qq-msf-worker", 0, 0, 0);
  prctl(PR_GET_NAME, name, 0, 0, 0);
  return (void *)(uintptr_t)(strcmp(name, "qq-msf-worker") != 0);
}

int main(void) {
  char first[8] = {0}, second[8] = {0}, name[16] = {0};
  FILE *one = fopen("/host-root/opt/QQ/wrapper.node", "rb");
  FILE *two = fopen64("/host-root/opt/QQ/wrapper.node", "rb");
  if (!one || !two) return 1;
  fread(first, 1, 7, one);
  fread(second, 1, 7, two);
  fclose(one);
  fclose(two);
  if (strcmp(first, "fixture") || strcmp(second, "fixture")) return 2;
  prctl(PR_SET_NAME, "qq-launcher", 0, 0, 0);
  prctl(PR_GET_NAME, name, 0, 0, 0);
  if (strcmp(name, "qq")) return 3;
  pthread_t thread;
  void *failed = NULL;
  if (pthread_create(&thread, NULL, check_thread_name, NULL) || pthread_join(thread, &failed) || failed) return 4;
  int source = 42, target = 0;
  struct iovec local = { &target, sizeof(target) };
  struct iovec remote = { &source, sizeof(source) };
  setenv("AMITIA_SNOWLUMA_PROC_MEM", "1", 1);
  if (process_vm_readv(getpid(), &local, 1, &remote, 1, 0) != sizeof(target) || target != source) return 5;
  return 0;
}
