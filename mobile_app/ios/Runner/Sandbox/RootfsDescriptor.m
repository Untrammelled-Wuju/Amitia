#import "RootfsDescriptor.h"

@implementation RootfsDescriptor

- (instancetype)initWithVersion:(NSString *)version
                   architecture:(NSString *)architecture
                  digestSHA256:(NSString *)digest
                       rootfsURL:(NSURL *)rootfsURL
                     sourceType:(RootfsSourceType)sourceType {
    self = [super init];
    if (self) {
        _version = [version copy];
        _architecture = [architecture copy];
        _digestSHA256 = [digest copy];
        _rootfsURL = rootfsURL;
        _sourceType = sourceType;
        _state = RootfsStateNotInstalled;
    }
    return self;
}

- (BOOL)isValidDescriptor {
    return (self.version.length > 0 &&
            self.architecture.length > 0 &&
            self.digestSHA256.length == 64 &&
            self.rootfsURL != nil);
}

- (BOOL)verifyLayoutPresent {
    if (!self.rootfsURL) return false;
    NSFileManager *fm = [NSFileManager defaultManager];
    NSArray *required = @[@"bin", @"etc", @"usr", @"var", @"tmp", @"home"];
    for (NSString *dir in required) {
        NSString *path = [self.rootfsURL.path stringByAppendingPathComponent:dir];
        BOOL isDir = NO;
        if (![fm fileExistsAtPath:path isDirectory:&isDir] || !isDir) {
            return NO;
        }
    }
    return YES;
}

+ (NSString *)stringFromState:(RootfsState)state {
    switch (state) {
        case RootfsStateUnknown: return @"unknown";
        case RootfsStateNotInstalled: return @"not_installed";
        case RootfsStateInstalling: return @"installing";
        case RootfsStateInstalled: return @"installed";
        case RootfsStateCorrupt: return @"corrupt";
        case RootfsStateFailed: return @"failed";
    }
    return @"unknown";
}

+ (NSString *)stringFromSourceType:(RootfsSourceType)type {
    switch (type) {
        case RootfsSourceTypeUnknown: return @"unknown";
        case RootfsSourceTypeBundled: return @"bundled";
        case RootfsSourceTypeRemoteOfficial: return @"remote_official";
        case RootfsSourceTypeUserImported: return @"user_imported";
        case RootfsSourceTypeLegacy: return @"legacy";
    }
    return @"unknown";
}

@end
