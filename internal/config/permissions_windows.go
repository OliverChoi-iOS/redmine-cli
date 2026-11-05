package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func currentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid, nil
}

func privateSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	// A protected DACL prevents the parent directory's grants from being inherited.
	return windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + sid.String() + ")")
}

func validateConfigDirPermissions(dir string, _ os.FileInfo) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	trusted := func(other *windows.SID) bool {
		return other != nil && (other.Equals(sid) || other.IsWellKnown(windows.WinLocalSystemSid) || other.IsWellKnown(windows.WinBuiltinAdministratorsSid))
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if !trusted(owner) {
		return fmt.Errorf("config parent directory must be owned by the current user, SYSTEM, or Administrators")
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil {
		return fmt.Errorf("config parent directory must have an access-control list")
	}
	// FILE_DELETE_CHILD (0x40) can remove even a file with a private DACL.
	const writeAccess = windows.GENERIC_ALL | windows.GENERIC_WRITE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.DELETE |
		windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES | 0x40
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		// Fail closed for conditional or object-specific grants that we cannot evaluate.
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("config parent directory has an unsupported access-control entry")
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Mask&writeAccess != 0 && !trusted(aceSID) {
			return fmt.Errorf("config parent directory must not grant write access to other users")
		}
	}
	return nil
}

func secureConfigFile(path string) error {
	sd, err := privateSecurityDescriptor()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

func createPrivateTemp(dir string) (*os.File, error) {
	sd, err := privateSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	attrs := windows.SecurityAttributes{
		Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd,
	}
	for i := 0; i < 10; i++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, fmt.Sprintf(".config.toml.%x.tmp", random))
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return nil, err
		}
		h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, &attrs,
			windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return os.NewFile(uintptr(h), path), nil
	}
	return nil, fmt.Errorf("could not create a unique private config file")
}
