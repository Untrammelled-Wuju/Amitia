#import <Foundation/Foundation.h>
#import "RootfsDescriptor.h"

NS_ASSUME_NONNULL_BEGIN

@interface RootfsResolver : NSObject

@property (nonatomic, copy, readonly) NSURL *rootfsBaseDirectory;
@property (nonatomic, copy, readonly) NSURL *versionsDirectory;
@property (nonatomic, copy, readonly) NSURL *activeManifestURL;
@property (nonatomic, copy, readonly) NSURL *stagingDirectory;

- (instancetype)initWithBaseDirectory:(NSURL *)baseDirectory;

- (nullable NSURL *)rootfsURLForVersion:(NSString *)version architecture:(NSString *)architecture;
- (nullable NSString *)manifestPathForVersion:(NSString *)version architecture:(NSString *)architecture;

- (nullable RootfsDescriptor *)resolveCurrentRootfs;
- (nullable RootfsDescriptor *)resolveInstalledRootfsVersion:(NSString *)version architecture:(NSString *)architecture;
- (NSArray<RootfsDescriptor *> *)listInstalledRootfs;
- (BOOL)isInstalledVersion:(NSString *)version architecture:(NSString *)architecture;

+ (NSURL *)defaultRootfsBaseDirectory;
+ (NSURL *)defaultVersionsDirectory;
+ (NSURL *)defaultActiveManifestURL;
+ (NSURL *)defaultStagingDirectory;

@end

NS_ASSUME_NONNULL_END
