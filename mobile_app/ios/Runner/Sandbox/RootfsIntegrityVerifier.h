#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, RootfsIntegrityResult) {
    RootfsIntegrityResultUnknown = 0,
    RootfsIntegrityResultMatch,
    RootfsIntegrityResultMismatch,
    RootfsIntegrityResultFileMissing,
    RootfsIntegrityResultAlgorithmUnavailable,
    RootfsIntegrityResultIOError,
    RootfsIntegrityResultDigestMalformed
};

@interface RootfsIntegrityVerifier : NSObject

+ (RootfsIntegrityResult)verifySHA256OfFileAtURL:(NSURL *)fileURL
                                   againstExpected:(NSString *)expectedDigest
                                             error:(NSError *_Nullable *_Nullable)error;

+ (RootfsIntegrityResult)verifySHA256OfData:(NSData *)data
                              againstExpected:(NSString *)expectedDigest;

+ (NSString *_Nullable)computeSHA256OfFileAtURL:(NSURL *)fileURL
                                          error:(NSError *_Nullable *_Nullable)error;

+ (BOOL)isValidSHA256Digest:(NSString *)digest;

@end

extern NSErrorDomain const RootfsIntegrityErrorDomain;

NS_ASSUME_NONNULL_END
