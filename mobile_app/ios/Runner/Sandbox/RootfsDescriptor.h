#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, RootfsState) {
    RootfsStateUnknown = 0,
    RootfsStateNotInstalled,
    RootfsStateInstalling,
    RootfsStateInstalled,
    RootfsStateCorrupt,
    RootfsStateFailed
};

typedef NS_ENUM(NSInteger, RootfsSourceType) {
    RootfsSourceTypeUnknown = 0,
    RootfsSourceTypeBundled,
    RootfsSourceTypeRemoteOfficial,
    RootfsSourceTypeUserImported,
    RootfsSourceTypeLegacy
};

@interface RootfsDescriptor : NSObject

@property (nonatomic, copy, readonly) NSString *version;
@property (nonatomic, copy, readonly) NSString *architecture;
@property (nonatomic, copy, readonly) NSString *digestSHA256;
@property (nonatomic, copy, readonly) NSURL *rootfsURL;
@property (nonatomic, readonly) RootfsSourceType sourceType;
@property (nonatomic, readonly) RootfsState state;
@property (nonatomic, copy, readonly, nullable) NSString *installMarkerPath;

- (instancetype)initWithVersion:(NSString *)version
                   architecture:(NSString *)architecture
                   digestSHA256:(NSString *)digest
                       rootfsURL:(NSURL *)rootfsURL
                     sourceType:(RootfsSourceType)sourceType;

- (BOOL)isValidDescriptor;
- (BOOL)verifyLayoutPresent;

+ (NSString *)stringFromState:(RootfsState)state;
+ (NSString *)stringFromSourceType:(RootfsSourceType)sourceType;

@end

NS_ASSUME_NONNULL_END
