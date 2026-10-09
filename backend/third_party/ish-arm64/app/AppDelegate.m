//
//  AppDelegate.m
//  iSH
//
//  Created by Theodore Dubois on 10/17/17.
//

#include <resolv.h>
#include <arpa/inet.h>
#include <netdb.h>
#import <SystemConfiguration/SystemConfiguration.h>
#import "AboutViewController.h"
#import "AppDelegate.h"
#import "AppGroup.h"
#import "CurrentRoot.h"
#import "ExceptionExfiltrator.h"
#import "iOSFS.h"
#import "SceneDelegate.h"
#import "PasteboardDevice.h"
#import "LocationDevice.h"
#import "NSObject+SaneKVO.h"
#import "Roots.h"
#import "TerminalViewController.h"
#import "UserPreferences.h"
#import "UIApplication+OpenURL.h"
#include "kernel/init.h"
#include "kernel/calls.h"
#include "kernel/mm.h"     // [T-ish-footprint-brake] memory governor feed
#import <mach/mach.h>       // task_info / TASK_VM_INFO (phys_footprint)
#import <os/proc.h>         // os_proc_available_memory
#include "fs/dyndev.h"
#include "fs/devices.h"
#include "fs/path.h"
#if defined(GUEST_ARM64) && defined(DEBUG)
#include "DebugServer.h"
#endif
#include "kernel/native_offload.h"
#ifdef ISH_FFMPEG_TEST
extern void native_builtins_init(void);
#endif

#if ISH_LINUX
#import "LinuxInterop.h"
#endif

@interface AppDelegate ()

@property BOOL exiting;
@property SCNetworkReachabilityRef reachability;

@end

#if !ISH_LINUX
static void ios_handle_exit(struct task *task, int code) {
    // we are interested in init and in children of init
    // this is called with pids_lock as an implementation side effect, please do not cite as an example of good API design
    if (task->parent != NULL && task->parent->parent != NULL)
        return;
    // pid should be saved now since task would be freed
    pid_t pid = task->pid;
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSNotificationCenter defaultCenter] postNotificationName:ProcessExitedNotification
                                                            object:nil
                                                          userInfo:@{@"pid": @(pid),
                                                                     @"code": @(code)}];
    });
}

static void ios_handle_die(const char *msg) {
    NSString *message = [NSString stringWithFormat:@"%s: %s", __func__, msg];
    iSHExceptionHandler([[NSException alloc] initWithName:NSGenericException reason:message userInfo:nil]);
}
#elif ISH_LINUX
void ReportPanic(const char *message) {
    [NSNotificationCenter.defaultCenter postNotificationName:KernelPanicNotification object:nil userInfo:@{@"message":@(message)}];
}
#endif

static int bootError;
static NSString *const kSkipStartupMessage = @"Skip Startup Message";

// [T-ish-footprint-brake] Feed the kernel's memory governor the two numbers
// jetsam actually operates on: the app's physical footprint and its remaining
// allowance. `limit = phys_footprint + os_proc_available_memory()` is the live
// per-app jetsam line, so there is no hardcoded per-device table and no
// boot-time snapshot that goes stale.
//
// Without this feed ish_footprint_mode() stays false and admission falls back
// to the fixed ANON_MMAP_LIMIT_PAGES ledger, which is not device-derived --
// V8's ~5x128MB startup cage commit then hits that ceiling regardless of how
// much memory the device actually has. The kernel side owns the state machine
// (BRAKE below 10% headroom, release above 15%, critical pressure forces it,
// and a feed older than 2s fails closed); this just reports the truth.
static void ish_memory_governor_tick(bool pressureCritical) {
    task_vm_info_data_t info;
    mach_msg_type_number_t count = TASK_VM_INFO_COUNT;
    uint64_t footprint = 0;
    if (task_info(mach_task_self(), TASK_VM_INFO, (task_info_t)&info, &count) == KERN_SUCCESS)
        footprint = info.phys_footprint;
    uint64_t avail = (uint64_t) os_proc_available_memory();
    if (footprint == 0 || avail == 0)
        return; // couldn't measure -- the kernel's staleness rule handles a dead feed
    ish_set_memory_status(footprint + avail, avail, pressureCritical);
}

static void ish_memory_governor_start(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        dispatch_queue_t q = dispatch_get_global_queue(QOS_CLASS_UTILITY, 0);
        // 250ms cadence: guest dirtying is bounded by emulation speed, so the
        // per-tick overshoot stays small against a 10%-of-limit brake margin.
        static dispatch_source_t timer;
        timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, q);
        dispatch_source_set_timer(timer, DISPATCH_TIME_NOW,
                                  250 * NSEC_PER_MSEC, 50 * NSEC_PER_MSEC);
        dispatch_source_set_event_handler(timer, ^{ ish_memory_governor_tick(false); });
        dispatch_resume(timer);
        // OS pressure events arrive faster than any poll -- treat CRITICAL as
        // an immediate brake regardless of our own arithmetic.
        static dispatch_source_t pressure;
        pressure = dispatch_source_create(DISPATCH_SOURCE_TYPE_MEMORYPRESSURE, 0,
                                          DISPATCH_MEMORYPRESSURE_WARN | DISPATCH_MEMORYPRESSURE_CRITICAL,
                                          q);
        dispatch_source_set_event_handler(pressure, ^{
            bool critical = (dispatch_source_get_data(pressure) & DISPATCH_MEMORYPRESSURE_CRITICAL) != 0;
            ish_memory_governor_tick(critical);
        });
        dispatch_resume(pressure);
    });
    // Synchronous first feed so footprint mode is active before the first
    // guest process ever runs.
    ish_memory_governor_tick(false);
}

// [T-ish-anon-cap-page-units] Install the legacy ledger cap from the device's
// real allowance. This stays as the fallback regime for the case where the
// governor can never measure (footprint mode never activates); once the first
// governor feed lands this stops being the admission control and only keeps
// counting for meminfo/diagnostics.
//
// Divide by the HOST page size, not the guest 4KB one: the counter is in guest
// pages but each committed guest page occupies a whole host page, so
// converting a host-byte budget with 4096 authorises 4x what was intended.
static void ish_install_anon_cap(void) {
    size_t avail = os_proc_available_memory();
    size_t hostPage = (size_t) getpagesize();
    if (avail == 0 || hostPage == 0) {
        NSLog(@"iSH: os_proc_available_memory unavailable -- keeping default anon cap");
        return;
    }
    static const double kGuestMemoryShare = 0.8;
    long pages = (long)((double) avail * kGuestMemoryShare / (double) hostPage);
    ish_set_anon_page_limit(pages);
    extern _Atomic long anon_page_limit;
    long effective = atomic_load(&anon_page_limit);
    double capMB = (double) effective * (double) hostPage / (1024 * 1024);
    NSLog(@"iSH: guest anon cap = %.0f MB host (%ld guest pages, host page %zuKB, "
          @"share %.0f%%, available %.0f MB)",
          capMB, effective, hostPage / 1024, kGuestMemoryShare * 100.0,
          (double) avail / (1024 * 1024));
}

@implementation AppDelegate

- (int)boot {
#if !ISH_LINUX
    // Install the device-derived ledger cap, then start the live governor.
    // Both must happen before the first guest process runs.
    ish_install_anon_cap();
    ish_memory_governor_start();

    NSURL *root = [Roots.instance rootUrl:Roots.instance.defaultRoot];

    int err = mount_root(&fakefs, [root URLByAppendingPathComponent:@"data"].fileSystemRepresentation);
    if (err < 0)
        return err;

    fs_register(&iosfs);
    fs_register(&iosfs_unsafe);

    // need to do this first so that we can have a valid current for the generic_mknod calls
    err = become_first_process();
    if (err < 0)
        return err;

#ifdef ISH_FFMPEG_TEST
    // Register built-in native handlers (fake_ffmpeg) for the ffmpeg test target.
    // This is NOT called in the standard iSH ARM64 target.
    native_builtins_init();
#endif

    FsInitialize();

    // create some device nodes
    // this will do nothing if they already exist
    generic_mknodat(AT_PWD, "/dev/tty1", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 1));
    generic_mknodat(AT_PWD, "/dev/tty2", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 2));
    generic_mknodat(AT_PWD, "/dev/tty3", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 3));
    generic_mknodat(AT_PWD, "/dev/tty4", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 4));
    generic_mknodat(AT_PWD, "/dev/tty5", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 5));
    generic_mknodat(AT_PWD, "/dev/tty6", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 6));
    generic_mknodat(AT_PWD, "/dev/tty7", S_IFCHR|0666, dev_make(TTY_CONSOLE_MAJOR, 7));

    generic_mknodat(AT_PWD, "/dev/tty", S_IFCHR|0666, dev_make(TTY_ALTERNATE_MAJOR, DEV_TTY_MINOR));
    generic_mknodat(AT_PWD, "/dev/console", S_IFCHR|0666, dev_make(TTY_ALTERNATE_MAJOR, DEV_CONSOLE_MINOR));
    generic_mknodat(AT_PWD, "/dev/ptmx", S_IFCHR|0666, dev_make(TTY_ALTERNATE_MAJOR, DEV_PTMX_MINOR));

    generic_mknodat(AT_PWD, "/dev/null", S_IFCHR|0666, dev_make(MEM_MAJOR, DEV_NULL_MINOR));
    generic_mknodat(AT_PWD, "/dev/zero", S_IFCHR|0666, dev_make(MEM_MAJOR, DEV_ZERO_MINOR));
    generic_mknodat(AT_PWD, "/dev/full", S_IFCHR|0666, dev_make(MEM_MAJOR, DEV_FULL_MINOR));
    generic_mknodat(AT_PWD, "/dev/random", S_IFCHR|0666, dev_make(MEM_MAJOR, DEV_RANDOM_MINOR));
    generic_mknodat(AT_PWD, "/dev/urandom", S_IFCHR|0666, dev_make(MEM_MAJOR, DEV_URANDOM_MINOR));
    
    generic_mkdirat(AT_PWD, "/dev/pts", 0755);
    
    // Permissions on / have been broken for a while, let's fix them
    generic_setattrat(AT_PWD, "/", (struct attr) {.type = attr_mode, .mode = 0755}, false);
    
    // Register clipboard device driver and create device node for it
    err = dyn_dev_register(&clipboard_dev, DEV_CHAR, DYN_DEV_MAJOR, DEV_CLIPBOARD_MINOR);
    if (err != 0) {
        return err;
    }
    generic_mknodat(AT_PWD, "/dev/clipboard", S_IFCHR|0666, dev_make(DYN_DEV_MAJOR, DEV_CLIPBOARD_MINOR));
    
    err = dyn_dev_register(&location_dev, DEV_CHAR, DYN_DEV_MAJOR, DEV_LOCATION_MINOR);
    if (err != 0)
        return err;
    generic_mknodat(AT_PWD, "/dev/location", S_IFCHR|0666, dev_make(DYN_DEV_MAJOR, DEV_LOCATION_MINOR));

    do_mount(&procfs, "proc", "/proc", "", 0);
    do_mount(&devptsfs, "devpts", "/dev/pts", "", 0);

    iosfs_init(); // let it mount any filesystems from user defaults

    [self configureDns];
    
    exit_hook = ios_handle_exit;
    die_handler = ios_handle_die;
#if !TARGET_OS_SIMULATOR
    NSString *sockTmp = [NSTemporaryDirectory() stringByAppendingString:@"ishsock"];
    sock_tmp_prefix = strdup(sockTmp.UTF8String);
#endif
    
    tty_drivers[TTY_CONSOLE_MAJOR] = &ios_console_driver;
    set_console_device(TTY_CONSOLE_MAJOR, 1);
    err = create_stdio("/dev/console", TTY_CONSOLE_MAJOR, 1);
    if (err < 0)
        return err;

#if defined(GUEST_ARM64) && defined(DEBUG)
    debug_server_start(1234);
#endif

    NSArray<NSString *> *command;
    command = UserPreferences.shared.bootCommand;
    char argv[4096];
    [Terminal convertCommand:command toArgs:argv limitSize:sizeof(argv)];
    const char *envp =
        "TERM=xterm-256color\0"
        "HOME=/root\0"
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\0"
#if defined(GUEST_ARM64)
        "PYTHONMALLOC=malloc\0"
#endif
        ;
    err = do_execve(command[0].UTF8String, command.count, argv, envp);
    if (err < 0)
        return err;
    task_start(current);

#else
    // On first launch, this will trigger the import of the default root. Make sure to do this before entering the kernel, because it needs to run something on the main thread, and that would deadlock.
    [Roots instance];
    NSArray<NSString *> *args = @[];
    actuate_kernel([args componentsJoinedByString:@" "].UTF8String);
#endif
    
    return 0;
}

#if ISH_LINUX
const char *DefaultRootPath() {
    return [Roots.instance rootUrl:Roots.instance.defaultRoot].fileSystemRepresentation;
}

void SyncHostname(void) {
    async_do_in_workqueue(^{
        char hostname[256];
        if (gethostname(hostname, sizeof(hostname)) < 0)
            return;
        linux_sethostname(hostname);
    });
}
#endif

- (void)configureDns {
#if !ISH_LINUX
    struct __res_state res;
    if (EXIT_SUCCESS != res_ninit(&res)) {
        exit(2);
    }
    NSMutableString *resolvConf = [NSMutableString new];
    if (res.dnsrch[0] != NULL) {
        [resolvConf appendString:@"search"];
        for (int i = 0; res.dnsrch[i] != NULL; i++) {
            [resolvConf appendFormat:@" %s", res.dnsrch[i]];
        }
        [resolvConf appendString:@"\n"];
    }
    union res_sockaddr_union servers[NI_MAXSERV];
    int serversFound = res_getservers(&res, servers, NI_MAXSERV);
    char address[NI_MAXHOST];
    for (int i = 0; i < serversFound; i ++) {
        union res_sockaddr_union s = servers[i];
        if (s.sin.sin_len == 0)
            continue;
        getnameinfo((struct sockaddr *) &s.sin, s.sin.sin_len,
                    address, sizeof(address),
                    NULL, 0, NI_NUMERICHOST);
        [resolvConf appendFormat:@"nameserver %s\n", address];
    }
    
    current = pid_get_task(1);
    struct fd *fd = generic_open("/etc/resolv.conf", O_WRONLY_ | O_CREAT_ | O_TRUNC_, 0666);
    if (!IS_ERR(fd)) {
        fd->ops->write(fd, resolvConf.UTF8String, [resolvConf lengthOfBytesUsingEncoding:NSUTF8StringEncoding]);
        fd_close(fd);
    }
#endif
}

+ (int)bootError {
    return bootError;
}

+ (void)maybePresentStartupMessageOnViewController:(UIViewController *)vc {
    if ([NSUserDefaults.standardUserDefaults integerForKey:kSkipStartupMessage] >= 1)
        return;
    if (!FsIsManaged()) {
        UIAlertController *alert = [UIAlertController alertControllerWithTitle:@"Install iSH’s built-in APK?"
                                                                       message:@"iSH now includes the APK package manager, but it must be manually activated."
                                                                preferredStyle:UIAlertControllerStyleAlert];
        [alert addAction:[UIAlertAction actionWithTitle:@"Show me how"
                                                  style:UIAlertActionStyleDefault
                                                handler:^(UIAlertAction * _Nonnull action) {
            [UIApplication openURL:@"https://go.ish.app/get-apk"];
        }]];
        [alert addAction:[UIAlertAction actionWithTitle:@"Don't show again"
                                                  style:UIAlertActionStyleDefault
                                                handler:nil]];
        [vc presentViewController:alert animated:YES completion:nil];
    }
    [NSUserDefaults.standardUserDefaults setInteger:1 forKey:kSkipStartupMessage];
}

- (BOOL)application:(UIApplication *)application willFinishLaunchingWithOptions:(NSDictionary<UIApplicationLaunchOptionsKey,id> *)launchOptions {
    NSUserDefaults *defaults = NSUserDefaults.standardUserDefaults;
    if ([defaults boolForKey:@"hail mary"]) {
        [defaults removeObjectForKey:kPreferenceBootCommandKey];
        [defaults removeObjectForKey:kPreferenceLaunchCommandKey];
        [defaults setBool:NO forKey:@"hail mary"];
    }
    if ([NSUserDefaults.standardUserDefaults boolForKey:@"recovery"])
        return YES;

    bootError = [self boot];

#if ISH_LINUX
    [NSNotificationCenter.defaultCenter addObserverForName:UIApplicationWillEnterForegroundNotification object:UIApplication.sharedApplication queue:nil usingBlock:^(NSNotification * _Nonnull note) {
        SyncHostname();
    }];
    SyncHostname();
#endif

    return YES;
}

void NetworkReachabilityCallback(SCNetworkReachabilityRef target, SCNetworkReachabilityFlags flags, void *info) {
    AppDelegate *self = (__bridge AppDelegate *) info;
    [self configureDns];
}

- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
    // get the network permissions popup to appear on chinese devices
    [[NSURLSession.sharedSession dataTaskWithURL:[NSURL URLWithString:@"http://captive.apple.com"]] resume];

    if ([NSUserDefaults.standardUserDefaults boolForKey:@"FASTLANE_SNAPSHOT"])
        [UIView setAnimationsEnabled:NO];

#if !ISH_LINUX
    NSString *ishVersion = [NSString stringWithFormat:@"iSH %@ (%@)",
                         [NSBundle.mainBundle objectForInfoDictionaryKey:@"CFBundleShortVersionString"],
                         [NSBundle.mainBundle objectForInfoDictionaryKey:(NSString *) kCFBundleVersionKey]];
    extern const char *proc_ish_version;
    proc_ish_version = strdup(ishVersion.UTF8String);
    // this defaults key is set when taking app store screenshots
    extern const char *uname_hostname_override;
    NSString *hostnameOverride = UserPreferences.shared._hostnameOverride;
    if (@available(iOS 16.0, *)) { // Hostname obfuscation is in effect
        hostnameOverride = hostnameOverride ? hostnameOverride : UserPreferences.shared.hostnameOverride;
    }
    if (hostnameOverride) {
        uname_hostname_override = strdup(hostnameOverride.UTF8String);
    }
#endif
    
    [UserPreferences.shared observe:@[@"shouldDisableDimming"] options:NSKeyValueObservingOptionInitial
                              owner:self usingBlock:^(typeof(self) self) {
        dispatch_async(dispatch_get_main_queue(), ^{
            UIApplication.sharedApplication.idleTimerDisabled = UserPreferences.shared.shouldDisableDimming;
        });
    }];
    
    // This code is IPv4 and IPv6 aware: see https://developer.apple.com/library/archive/samplecode/Reachability/Listings/ReadMe_md.html
    struct sockaddr_in address = {
        .sin_len = sizeof(address),
        .sin_family = AF_INET,
    };
    self.reachability = SCNetworkReachabilityCreateWithAddress(kCFAllocatorDefault, (struct sockaddr *) &address);
    SCNetworkReachabilityContext context = {
        .info = (__bridge void *) self,
    };
    SCNetworkReachabilitySetCallback(self.reachability, NetworkReachabilityCallback, &context);
    SCNetworkReachabilityScheduleWithRunLoop(self.reachability, CFRunLoopGetMain(), kCFRunLoopCommonModes);

    if (self.window != nil) {
        // For iOS <13, where the app delegate owns the window instead of the scene
        if ([NSUserDefaults.standardUserDefaults boolForKey:@"recovery"]) {
            UINavigationController *vc = [[UIStoryboard storyboardWithName:@"About" bundle:nil] instantiateInitialViewController];
            AboutViewController *avc = (AboutViewController *) vc.topViewController;
            avc.recoveryMode = YES;
            self.window.rootViewController = vc;
            return YES;
        }
        TerminalViewController *vc = (TerminalViewController *) self.window.rootViewController;
        currentTerminalViewController = vc;
        [vc startNewSession];
    }
    return YES;
}

- (void)application:(UIApplication *)application didDiscardSceneSessions:(NSSet<UISceneSession *> *)sceneSessions API_AVAILABLE(ios(13.0)) {
    for (UISceneSession *sceneSession in sceneSessions) {
        NSString *terminalUUID = sceneSession.stateRestorationActivity.userInfo[@"TerminalUUID"];
        [[Terminal terminalWithUUID:[[NSUUID alloc] initWithUUIDString:terminalUUID]] destroy];
    }
}

- (void)dealloc {
    if (self.reachability != NULL) {
        SCNetworkReachabilityUnscheduleFromRunLoop(self.reachability, CFRunLoopGetMain(), kCFRunLoopCommonModes);
        CFRelease(self.reachability);
    }
}

- (void)exitApp {
    self.exiting = YES;
    id app = [UIApplication sharedApplication];
    [app suspend];
}

- (void)applicationDidEnterBackground:(UIApplication *)application {
    if (self.exiting)
        exit(0);
}

@end

#if !ISH_LINUX
NSString *const ProcessExitedNotification = @"ProcessExitedNotification";
#else
NSString *const KernelPanicNotification = @"KernelPanicNotification";
#endif
