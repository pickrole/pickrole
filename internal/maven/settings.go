// Package maven stores the CodeArtifact token in Maven's settings.xml.
//
// The file is edited as text, not re-serialised, so the user's formatting,
// comments and every other entry stay exactly as they were: only the
// <server> whose <id> matches is touched.
package maven

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/pickrole/pickrole/internal/config"
	"github.com/pickrole/pickrole/internal/fsutil"
	"github.com/pickrole/pickrole/internal/i18n"
)

// Username is what CodeArtifact expects together with the token.
const Username = "aws"

const newSettings = `<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.2.0 https://maven.apache.org/xsd/settings-1.2.0.xsd">
  <servers>
  </servers>
</settings>
`

var (
	serverBlockRe = regexp.MustCompile(`(?s)<server>.*?</server>`)
	passwordRe    = regexp.MustCompile(`(?s)<password>.*?</password>`)
	usernameRe    = regexp.MustCompile(`(?s)<username>.*?</username>`)
	serversOpenRe = regexp.MustCompile(`<servers\s*/>`)
	commentRe     = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// ErrInvalidSettings means the file could not be edited safely.
var ErrInvalidSettings = i18n.New("maven.invalid_settings")

// UpsertServer sets the credentials of the <server> with the given id,
// creating the file, the <servers> element or the <server> entry as needed.
// A one-time backup (settings.xml.pickrole.bak) is kept the first time an
// existing file is changed.
func UpsertServer(path, serverID, token string) error {
	// Checked here, where the token is written, so no caller can skip it.
	if err := config.ValidateSettingsPath(path); err != nil {
		return err
	}
	path = fsutil.ExpandHome(path)
	if strings.TrimSpace(serverID) == "" {
		return i18n.New("maven.empty_server_id")
	}
	existing, err := os.ReadFile(path) // #nosec G304 -- validated above: inside the home directory
	fresh := errors.Is(err, fs.ErrNotExist)
	if err != nil && !fresh {
		return err
	}
	content := string(existing)
	if fresh {
		content = newSettings
	}
	updated, err := upsert(content, serverID, token)
	if err != nil {
		return err
	}
	if !fresh && updated != content {
		if err := writeBackup(path+".pickrole.bak", existing); err != nil {
			return i18n.Wrap("maven.backup", err)
		}
	}
	// Always 0600, even if the file was readable by others: it now holds a
	// token, and Maven only needs its owner to read it.
	return fsutil.WriteFileAtomic(path, []byte(updated), 0o600)
}

// writeBackup keeps the original file, once. O_EXCL fails on any existing
// name without following it, so a link planted as the backup cannot send
// the old content (and any old token in it) somewhere else.
func writeBackup(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- next to the validated settings file
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func upsert(content, serverID, token string) (string, error) {
	idRe := regexp.MustCompile(`<id>\s*` + regexp.QuoteMeta(serverID) + `\s*</id>`)
	password := "<password>" + escape(token) + "</password>"
	username := "<username>" + Username + "</username>"

	// Anything inside <!-- --> is ignored: a commented-out example with our
	// id must not receive the token instead of the real entry.
	comments := commentRe.FindAllStringIndex(content, -1)

	// 1. An existing <server> with our id: replace its credentials in place.
	for _, loc := range serverBlockRe.FindAllStringIndex(content, -1) {
		block := content[loc[0]:loc[1]]
		if inComment(comments, loc[0]) || !idRe.MatchString(block) {
			continue
		}
		nb := block
		if usernameRe.MatchString(nb) {
			nb = usernameRe.ReplaceAllLiteralString(nb, username)
		} else {
			nb = strings.Replace(nb, "</server>", "  "+username+"\n    </server>", 1)
		}
		if passwordRe.MatchString(nb) {
			nb = passwordRe.ReplaceAllLiteralString(nb, password)
		} else {
			nb = strings.Replace(nb, "</server>", "  "+password+"\n    </server>", 1)
		}
		return content[:loc[0]] + nb + content[loc[1]:], nil
	}

	entry := "    <server>\n" +
		"      <id>" + escape(serverID) + "</id>\n" +
		"      " + username + "\n" +
		"      " + password + "\n" +
		"    </server>\n"

	// 2. A <servers> element: add our entry at its end.
	if i := lastIndex(content, "</servers>", comments); i >= 0 {
		lineStart := strings.LastIndex(content[:i], "\n") + 1
		return content[:lineStart] + entry + content[lineStart:], nil
	}
	// 3. A self-closing <servers/>.
	for _, loc := range serversOpenRe.FindAllStringIndex(content, -1) {
		if !inComment(comments, loc[0]) {
			return content[:loc[0]] + "<servers>\n" + entry + "  </servers>" + content[loc[1]:], nil
		}
	}
	// 4. No <servers> at all: add one before </settings>.
	i := lastIndex(content, "</settings>", comments)
	if i < 0 {
		return "", ErrInvalidSettings
	}
	lineStart := strings.LastIndex(content[:i], "\n") + 1
	return content[:lineStart] + "  <servers>\n" + entry + "  </servers>\n" + content[lineStart:], nil
}

// inComment reports whether offset i falls inside one of the comments.
func inComment(comments [][]int, i int) bool {
	for _, c := range comments {
		if i >= c[0] && i < c[1] {
			return true
		}
	}
	return false
}

// lastIndex is strings.LastIndex, skipping matches inside comments.
func lastIndex(content, sub string, comments [][]int) int {
	end := len(content)
	for {
		i := strings.LastIndex(content[:end], sub)
		if i < 0 || !inComment(comments, i) {
			return i
		}
		end = i
	}
}
