package config

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func assertPrivateDescriptor(t *testing.T, sd *windows.SECURITY_DESCRIPTOR) {
	t.Helper()
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("DACL is not protected: control=%x err=%v", control, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("DACL missing: %v", err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("DACL contains %d grants, want one current-user grant", dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	const required = windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE | windows.DELETE | windows.WRITE_DAC
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERITED_ACE != 0 || !aceSID.Equals(sid) || ace.Mask&required != required {
		t.Fatalf("DACL = %s, want read/write access only for %s", sd.String(), sid.String())
	}
}

func assertPrivateConfigFile(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateDescriptor(t, sd)
}

func setTestDACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateTempHasProtectedDACLBeforeWriting(t *testing.T) {
	f, err := createPrivateTemp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateDescriptor(t, sd)
}

func TestWriteFileRepairsWindowsDACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	setTestDACL(t, path, "D:P(A;;FA;;;"+sid.String()+")(A;;FR;;;WD)")
	if err := WriteFile(path, Defaults()); err != nil {
		t.Fatal(err)
	}
	assertPrivateConfigFile(t, path)
}

func TestWindowsConfigParentPermissions(t *testing.T) {
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, extra string
		wantError   bool
	}{
		{"private", "", false},
		{"read-only inheritance", "(A;OICI;FR;;;WD)", false},
		{"everyone write", "(A;;FW;;;WD)", true},
		{"everyone delete child", "(A;;0x40;;;WD)", true},
		{"everyone change ACL", "(A;;WD;;;WD)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			setTestDACL(t, dir, "D:P(A;OICI;FA;;;"+sid.String()+")"+tc.extra)
			path := filepath.Join(dir, "config.toml")
			err := WriteFile(path, Defaults())
			if (err != nil) != tc.wantError {
				t.Fatalf("WriteFile error = %v, wantError=%t", err, tc.wantError)
			}
			if !tc.wantError {
				assertPrivateConfigFile(t, path)
			}
		})
	}
}

func TestWindowsDefaultConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "redmine-cli", "config.toml"); got != want {
		t.Fatalf("config path = %q, want %q", got, want)
	}
}
