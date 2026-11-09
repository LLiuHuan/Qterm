//go:build darwin && cgo

package biometric

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=12.0
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include <dispatch/dispatch.h>

// 注意：cgo 默认按 MRC（非 ARC）编译 Objective-C，因此这里显式 retain/release，
// 不能写成 ARC 语法（`__bridge`、`weak` 等）。

// qterm_biometric_available 判断本机能否做生物识别（有 Touch ID 硬件且已录入指纹）。
static int qterm_biometric_available(void) {
	LAContext *context = [[LAContext alloc] init];
	BOOL available = [context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
					      error:NULL];
	// 一次判定后立刻释放：它只回答"能不能"，不参与后续授权。
	[context release];
	return available ? 1 : 0;
}

// qterm_biometric_unlock 弹一次系统指纹框并等待用户操作。
//
// 返回值：
//   0 = 认证通过
//   1 = 用户取消或认证失败（含多次不匹配后的锁定）
//   2 = 本机不支持生物识别（没有硬件或未录入指纹）
//
// 线程约定：在调用方 goroutine 上同步等待，**不要**把它派发到主线程再阻塞主线程——
// LocalAuthentication 的 reply 可能被派回主线程，主线程一旦被信号量卡住就会死锁。
// 系统授权框由授权服务自己呈现，不要求调用线程是主线程。
static int qterm_biometric_unlock(const char *reason) {
	LAContext *context = [[LAContext alloc] init];
	NSError *canEvaluateError = nil;
	if (![context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
				  error:&canEvaluateError]) {
		[context release];
		return 2;
	}

	// 用 alloc/init 拿一个 +1 的对象：授权框要一直用到回调结束，
	// 不能交给自动释放池（Go 的 C 线程没有现成的池，autorelease 等于泄漏）。
	NSString *localizedReason = [[NSString alloc] initWithUTF8String:(reason != NULL ? reason : "")];
	dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
	__block BOOL success = NO;

	[context evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
		localizedReason:localizedReason
			  reply:^(BOOL ok, NSError *_Nullable error) {
				success = ok;
				dispatch_semaphore_signal(semaphore);
			  }];

	// 不设内部超时：系统授权框可以一直等用户操作（含「使用密码」回退）。
	// 上限由 Go 侧的 settleTimeout 兜底，避免界面永远停在"正在验证"。
	dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);

	[localizedReason release];
	[context release];
	return success ? 0 : 1;
}
*/
import "C"

import (
	"context"
	"time"
	"unsafe"
)

const settleTimeout = 5 * time.Minute

func Available() bool {
	result := make(chan bool, 1)
	go func() {
		result <- C.qterm_biometric_available() == 1
	}()
	select {
	case available := <-result:
		return available
	case <-time.After(settleTimeout):
		return false
	}
}

func Unlock(ctx context.Context, reason string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	result := make(chan int, 1)
	go func() {

		cReason := C.CString(reason)
		defer C.free(unsafe.Pointer(cReason))
		result <- int(C.qterm_biometric_unlock(cReason))
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case code := <-result:
		switch code {
		case 0:
			return nil
		case 2:
			return ErrUnavailable
		default:
			return ErrNotVerified
		}
	case <-time.After(settleTimeout):
		return ErrTimeout
	}
}
