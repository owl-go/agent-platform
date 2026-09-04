package workspacefs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nativeStateSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func NativeSessionStatePath(root, ownerID, sessionID, runtime string) (string, error) {
	for name, value := range map[string]string{"owner ID": ownerID, "session ID": sessionID, "Runtime": runtime} {
		if !nativeStateSegment.MatchString(value) || value == "." || value == ".." {
			return "", fmt.Errorf("%s is invalid for native Runtime state", name)
		}
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) || root == string(filepath.Separator) {
		return "", fmt.Errorf("Workspace root must be an absolute, non-root path")
	}
	path := filepath.Join(root, ".native-session-state", ownerID, sessionID, runtime)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("native Runtime state escapes the Workspace root")
	}
	return path, nil
}

func NativeExpertSessionStatePath(root, ownerID, sessionID, expertID string, expertVersion int64, runtime string) (string, error) {
	identity := fmt.Sprintf("expert-%s-v%d-%s", expertID, expertVersion, runtime)
	return NativeSessionStatePath(root, ownerID, sessionID, identity)
}

func NativeExpertRunConversationStatePath(root, ownerID, conversationID, expertID string, expertVersion int64, runtime string) (string, error) {
	identity := fmt.Sprintf("expert-%s-v%d-%s", expertID, expertVersion, runtime)
	return nativeConversationStatePath(root, ".native-run-conversation-state", ownerID, conversationID, identity)
}

func RemoveNativeSessionState(root, ownerID, sessionID string) error {
	path, err := NativeSessionStatePath(root, ownerID, sessionID, "runtime")
	if err != nil {
		return err
	}
	path = filepath.Dir(path)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove native Runtime state: %w", err)
	}
	return nil
}

func nativeConversationStatePath(root, namespace, ownerID, conversationID, runtime string) (string, error) {
	for name, value := range map[string]string{"owner ID": ownerID, "conversation ID": conversationID, "Runtime": runtime} {
		if !nativeStateSegment.MatchString(value) || value == "." || value == ".." {
			return "", fmt.Errorf("%s is invalid for native Runtime state", name)
		}
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) || root == string(filepath.Separator) {
		return "", fmt.Errorf("Workspace root must be an absolute, non-root path")
	}
	path := filepath.Join(root, namespace, ownerID, conversationID, runtime)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("native Runtime state escapes the Workspace root")
	}
	return path, nil
}
