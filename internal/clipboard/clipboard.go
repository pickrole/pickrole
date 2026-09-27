// Package clipboard copies secrets so the OS does not keep them around.
package clipboard

import "github.com/pickrole/pickrole/internal/i18n"

// ErrUnsupported means this platform has no way to mark clipboard content
// as sensitive; the caller should fall back to a plain copy.
var ErrUnsupported = i18n.New("clipboard.unsupported")
