#import "RootfsIntegrityVerifier.h"
#import <CommonCrypto/CommonDigest.h>

@implementation RootfsIntegrityVerifier

+ (RootfsIntegrityResult)verifySHA256OfFileAtURL:(NSURL *)fileURL
                                againstExpected:(NSString *)expectedDigest
                                          error:(NSError *_Nullable *_Nullable)error {
    if (!fileURL || !fileURL.isFileURL) {
        if (error) *error = [NSError errorWithDomain:@"RootfsIntegrity" code:RootfsIntegrityResultFileMissing userInfo:nil];
        return RootfsIntegrityResultFileMissing;
    }
    if (![[NSFileManager defaultManager] fileExistsAtPath:fileURL.path]) {
        if (error) *error = [NSError errorWithDomain:@"RootfsIntegrity" code:RootfsIntegrityResultFileMissing userInfo:nil];
        return RootfsIntegrityResultFileMissing;
    }
    if (!expectedDigest || expectedDigest.length != 64) {
        if (error) *error = [NSError errorWithDomain:@"RootfsIntegrity" code:RootfsIntegrityResultAlgorithmUnavailable userInfo:nil];
        return RootfsIntegrityResultAlgorithmUnavailable;
    }

    NSError *readError = nil;
    NSData *data = [NSData dataWithContentsOfURL:fileURL options:NSDataReadingMappedIfSafe error:&readError];
    if (!data) {
        if (error) *error = readError;
        return RootfsIntegrityResultIOError;
    }

    NSString *actual = [self sha256OfData:data];
    if ([actual isEqualToString:[expectedDigest lowercaseString]]) {
        return RootfsIntegrityResultMatch;
    }
    return RootfsIntegrityResultMismatch;
}

+ (RootfsIntegrityResult)verifySHA256OfData:(NSData *)data
                           againstExpected:(NSString *)expectedDigest {
    if (!data || data.length == 0) return RootfsIntegrityResultIOError;
    if (!expectedDigest || expectedDigest.length != 64) return RootfsIntegrityResultAlgorithmUnavailable;
    NSString *actual = [self sha256OfData:data];
    return [actual isEqualToString:[expectedDigest lowercaseString]] ? RootfsIntegrityResultMatch : RootfsIntegrityResultMismatch;
}

+ (NSString *_Nullable)computeSHA256OfFileAtURL:(NSURL *)fileURL
                                          error:(NSError *_Nullable *_Nullable)error {
    if (!fileURL || !fileURL.isFileURL) {
        if (error) *error = [NSError errorWithDomain:@"RootfsIntegrity" code:RootfsIntegrityResultFileMissing userInfo:nil];
        return nil;
    }
    NSData *data = [NSData dataWithContentsOfURL:fileURL options:0 error:error];
    if (!data) return nil;
    return [self sha256OfData:data];
}

+ (NSString *)sha256OfData:(NSData *)data {
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CC_SHA256(data.bytes, (CC_LONG)data.length, digest);
    NSMutableString *out = [NSMutableString stringWithCapacity:CC_SHA256_DIGEST_LENGTH * 2];
    for (int i = 0; i < CC_SHA256_DIGEST_LENGTH; i++) {
        [out appendFormat:@"%02x", digest[i]];
    }
    return [out copy];
}

@end
