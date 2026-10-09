#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

@interface IOSBusinessRuntimeProcess : NSObject
@property(nonatomic, readonly) uint64_t generation;
@property(nonatomic, readonly) int stdoutDescriptor;
@property(nonatomic, readonly) int stderrDescriptor;
@property(nonatomic, readonly) int hostBridgeDescriptor;
- (BOOL)spawn:(NSArray<NSString *> *)arguments environment:(NSDictionary<NSString *, NSString *> *)environment error:(NSError **)error __attribute__((swift_error(none)));
- (BOOL)isRunning:(NSError **)error __attribute__((swift_error(none)));
- (BOOL)cancel:(NSError **)error __attribute__((swift_error(none)));
- (BOOL)reap:(NSError **)error __attribute__((swift_error(none)));
@end

NS_ASSUME_NONNULL_END
