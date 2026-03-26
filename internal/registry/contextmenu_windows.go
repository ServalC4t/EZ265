//go:build windows

package registry

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	shellKeyName = `*\shell\H265Convert`
	menuLabel    = "H265一発変換"
)

// IsRegistered checks if the context menu entry exists.
func IsRegistered() bool {
	key, err := registry.OpenKey(registry.CLASSES_ROOT, shellKeyName, registry.READ)
	if err == nil {
		key.Close()
		return true
	}
	hkcuPath := `Software\Classes\` + shellKeyName
	key, err = registry.OpenKey(registry.CURRENT_USER, hkcuPath, registry.READ)
	if err == nil {
		key.Close()
		return true
	}
	return false
}

// Register adds the cascading context menu entry with two sub-items.
func Register() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine exe path: %w", err)
	}

	// Try HKCR (requires admin)
	if err := registerUnder(registry.CLASSES_ROOT, shellKeyName, exePath); err == nil {
		return nil
	}

	// Fallback to HKCU
	hkcuPath := `Software\Classes\` + shellKeyName
	return registerUnder(registry.CURRENT_USER, hkcuPath, exePath)
}

func registerUnder(root registry.Key, keyPath, exePath string) error {
	// Create parent key: H265一発変換 (cascading menu)
	key, _, err := registry.CreateKey(root, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetStringValue("MUIVerb", menuLabel); err != nil {
		return err
	}
	if err := key.SetStringValue("Icon", fmt.Sprintf("%s,0", exePath)); err != nil {
		return err
	}
	// SubCommands="" tells Explorer to look for sub-items under \shell
	if err := key.SetStringValue("SubCommands", ""); err != nil {
		return err
	}

	// Sub-item 1: 動画を追加
	addKeyPath := keyPath + `\shell\01add`
	addKey, _, err := registry.CreateKey(root, addKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer addKey.Close()
	if err := addKey.SetStringValue("", "動画を追加"); err != nil {
		return err
	}
	if err := addKey.SetStringValue("Icon", fmt.Sprintf("%s,0", exePath)); err != nil {
		return err
	}

	addCmdKey, _, err := registry.CreateKey(root, addKeyPath+`\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer addCmdKey.Close()
	if err := addCmdKey.SetStringValue("", fmt.Sprintf(`"%s" --add "%%1"`, exePath)); err != nil {
		return err
	}

	// Sub-item 2: 動画を追加してスタート
	startKeyPath := keyPath + `\shell\02start`
	startKey, _, err := registry.CreateKey(root, startKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer startKey.Close()
	if err := startKey.SetStringValue("", "動画を追加してスタート"); err != nil {
		return err
	}
	if err := startKey.SetStringValue("Icon", fmt.Sprintf("%s,0", exePath)); err != nil {
		return err
	}

	startCmdKey, _, err := registry.CreateKey(root, startKeyPath+`\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer startCmdKey.Close()
	return startCmdKey.SetStringValue("", fmt.Sprintf(`"%s" --encode "%%1"`, exePath))
}

// Unregister removes the context menu entry.
func Unregister() error {
	errCR := deleteKeyTree(registry.CLASSES_ROOT, shellKeyName)
	hkcuPath := `Software\Classes\` + shellKeyName
	errCU := deleteKeyTree(registry.CURRENT_USER, hkcuPath)

	if errCR != nil && errCU != nil {
		return fmt.Errorf("HKCR: %v; HKCU: %v", errCR, errCU)
	}
	return nil
}

func deleteKeyTree(root registry.Key, keyPath string) error {
	// Delete deepest keys first
	subkeys := []string{
		keyPath + `\shell\01add\command`,
		keyPath + `\shell\01add`,
		keyPath + `\shell\02start\command`,
		keyPath + `\shell\02start`,
		keyPath + `\shell`,
		// Also clean up old single-command key if present
		keyPath + `\command`,
	}
	for _, sk := range subkeys {
		registry.DeleteKey(root, sk)
	}
	return registry.DeleteKey(root, keyPath)
}
