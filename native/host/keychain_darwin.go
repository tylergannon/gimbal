package main

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

static CFMutableDictionaryRef sessionQuery(const char *account) {
    CFMutableDictionaryRef query = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFStringRef key = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
    CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
    CFDictionarySetValue(query, kSecAttrService, CFSTR("com.tylergannon.gimbal.session"));
    CFDictionarySetValue(query, kSecAttrAccount, key);
    CFRelease(key);
    return query;
}

static OSStatus loadSession(const char *account, void **bytes, int *length) {
    CFMutableDictionaryRef query = sessionQuery(account);
    CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
    CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
    CFDataRef result = NULL;
    OSStatus status = SecItemCopyMatching(query, (CFTypeRef *)&result);
    CFRelease(query);
    if (status == errSecSuccess) {
        *length = (int)CFDataGetLength(result);
        *bytes = malloc(*length);
        if (*length > 0 && !*bytes) { CFRelease(result); return errSecAllocate; }
        memcpy(*bytes, CFDataGetBytePtr(result), *length);
        CFRelease(result);
    }
    return status;
}

static OSStatus saveSession(const char *account, const void *bytes, int length) {
    CFMutableDictionaryRef query = sessionQuery(account);
    if (!bytes) {
        OSStatus status = SecItemDelete(query);
        CFRelease(query);
        return status == errSecItemNotFound ? errSecSuccess : status;
    }
    CFDataRef value = CFDataCreate(NULL, bytes, length);
    CFMutableDictionaryRef attrs = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(attrs, kSecValueData, value);
    CFDictionarySetValue(attrs, kSecAttrAccessible, kSecAttrAccessibleWhenUnlockedThisDeviceOnly);
    OSStatus status = SecItemUpdate(query, attrs);
    if (status == errSecItemNotFound) {
        CFDictionarySetValue(query, kSecValueData, value);
        CFDictionarySetValue(query, kSecAttrAccessible, kSecAttrAccessibleWhenUnlockedThisDeviceOnly);
        status = SecItemAdd(query, NULL);
    }
    CFRelease(attrs);
    CFRelease(value);
    CFRelease(query);
    return status;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type keychainStore struct{ account string }

func (s keychainStore) load() ([]byte, error) {
	account := C.CString(s.account)
	defer C.free(unsafe.Pointer(account))
	var bytes unsafe.Pointer
	var length C.int
	status := C.loadSession(account, &bytes, &length)
	if status == C.errSecItemNotFound {
		return nil, nil
	}
	if status != C.errSecSuccess {
		return nil, fmt.Errorf("read desktop login from Keychain: status %d", status)
	}
	defer C.free(bytes)
	return C.GoBytes(bytes, length), nil
}

func (s keychainStore) save(raw []byte) error {
	account := C.CString(s.account)
	defer C.free(unsafe.Pointer(account))
	var bytes unsafe.Pointer
	if len(raw) > 0 {
		bytes = C.CBytes(raw)
		defer C.free(bytes)
	}
	if status := C.saveSession(account, bytes, C.int(len(raw))); status != C.errSecSuccess {
		return fmt.Errorf("save desktop login in Keychain: status %d", status)
	}
	return nil
}
