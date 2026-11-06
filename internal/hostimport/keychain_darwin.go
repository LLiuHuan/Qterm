//go:build darwin && cgo

package hostimport

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

// qterm_keychain_read 用 Security.framework 直接读通用密码条目。
//
// 为什么不 shell 出去调 `security`：那样系统授权框会显示"security 想要使用…"，
// 用户不知道该不该允许。在进程内调用 Security.framework，授权框显示的是本应用名
// （Qterm），语义正确。
static OSStatus qterm_keychain_read(const char *service, const char *account,
                                    void **outData, UInt32 *outLength) {
	return SecKeychainFindGenericPassword(
		NULL,
		(UInt32)strlen(service), service,
		(UInt32)strlen(account), account,
		outLength, outData, NULL);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func darwinKeychainPassword(service, account string) (string, error) {
	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))

	var data unsafe.Pointer
	var length C.UInt32

	status := C.qterm_keychain_read(cService, cAccount, &data, &length)
	if status != 0 {
		if int32(status) == -25300 {
			return "", errKeychainItemNotFound
		}
		return "", errors.New("keychain read failed")
	}
	defer C.SecKeychainItemFreeContent(nil, data)
	if data == nil || length == 0 {
		return "", errKeychainItemNotFound
	}
	value := C.GoStringN((*C.char)(data), C.int(length))
	return value, nil
}

var errKeychainItemNotFound = errors.New("keychain item not found")
