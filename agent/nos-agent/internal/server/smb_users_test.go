package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMBUserCreate_InvalidUsername(t *testing.T) {
	body := SMBUserCreateRequest{Username: "bad name"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/smb/user-create", bytes.NewReader(b))
	w := httptest.NewRecorder()
	handleSMBUserCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSMBUsersList_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/smb/users", nil)
	w := httptest.NewRecorder()
	handleSMBUsersList(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

type fakeSMBAccounts struct {
	comment, smb map[string]string
	regular      map[string]bool
	ran          []string
}

func (f *fakeSMBAccounts) Run(name string, args ...string) ([]byte, error) {
	f.ran = append(f.ran, name+" "+strings.Join(args, " "))
	if name == "pdbedit" {
		return []byte(f.smb[args[1]]), nil
	}
	return nil, nil
}

func (f *fakeSMBAccounts) Lookup(name string) (string, bool, bool) {
	c, ok := f.comment[name]
	return c, f.regular[name], ok
}

// Deleting a nOS user removes their SMB password, and their Unix account
// only when nOS created it.
func TestDeleteSMBUser(t *testing.T) {
	f := &fakeSMBAccounts{
		comment: map[string]string{"bob": nosAccountComment, "admin": "Console admin", "daemon": ""},
		smb:     map[string]string{"bob": "bob:1000:", "admin": "admin:1001:"},
		regular: map[string]bool{"bob": true, "admin": true},
	}
	if removed, err := deleteSMBUser(f, "bob"); err != nil || !removed {
		t.Fatalf("bob: %v %v", removed, err)
	}
	if removed, err := deleteSMBUser(f, "admin"); err != nil || removed {
		t.Fatalf("the console account must stay: %v %v", removed, err)
	}
	if _, err := deleteSMBUser(f, "daemon"); err == nil {
		t.Fatal("a system account was accepted")
	}
	if removed, err := deleteSMBUser(f, "ghost"); err != nil || removed {
		t.Fatalf("an unknown name is a no-op: %v %v", removed, err)
	}
	got := strings.Join(f.ran, "; ")
	for _, want := range []string{"smbpasswd -x bob", "userdel bob", "smbpasswd -x admin"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "userdel admin") {
		t.Errorf("deleted the console account: %s", got)
	}
}
