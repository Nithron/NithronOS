package sync

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrOutsideShare is a path that would leave its share.
var ErrOutsideShare = errors.New("path is outside the share")

// ResolveInShare turns a client's share-relative path into a path under
// sharePath. ".." can't climb above the share, and the deepest existing part
// of the result must resolve (symlinks followed) inside the share. (Paths
// were joined onto the share as given, so "../../../etc/nos/users.json"
// reached nosd's own files.)
func ResolveInShare(sharePath, rel string) (string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", ErrOutsideShare
	}
	full := filepath.Join(sharePath, filepath.Clean("/"+rel))
	root, err := filepath.EvalSymlinks(sharePath)
	if err != nil {
		return "", err
	}
	for p := full; ; {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			if real != root && !strings.HasPrefix(real, root+string(filepath.Separator)) {
				return "", ErrOutsideShare
			}
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return full, nil
}
