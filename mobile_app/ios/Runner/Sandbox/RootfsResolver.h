#import <Foundation/Foundation.h>
#import "RootfsDescriptor.h"

NS_ASSUME_NONNULL_BEGIN

@interface RootfsResolver : NSObject

@property (nonatomic, copy, readonly) NSURL *rootfsBaseDirectory;
@property (nonatomic, copy, readonly) NSURL *stagingDirectory;
@property (nonatomic, copy, readonly) NSURL *installMarkerURL;

- (instancetype)initWithBaseDirectory:(NSURL *)baseDirectory;

- (NSURL *)rootfsURLForVersion:(NSString *)version architecture:(NSString *)architecture;
- (NSURL *)installMarkerURLForVersion:(NSString *)version architecture:(NSString *)architecture;

- (RootfsDescriptor *_Nullable)resolveCurrentRootfs;
- (NSArray<RootfsDescriptor *> *)listInstalledRootfs;
- (BOOL)isInstalledVersion:(NSString *)version architecture:(NSString *)architecture;

+ (NSURL *)defaultRootfsBaseDirectory;
+ (NSURL *)defaultStagingDirectory;
+ (NSURL *)defaultInstallMarkerURL;

@end

NS_ASSUME_NONNULL_END
