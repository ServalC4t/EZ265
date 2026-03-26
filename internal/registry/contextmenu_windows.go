//go:build windows

package registry

import (
	"fmt"
	"os"

	"h265conv/internal/i18n"

	"golang.org/x/sys/windows/registry"
)

const (
	shellKeyName = `*\shell\H265Convert`
)

// IsRegistered checks if the context menu entry exists.
func IsRegistered() bool {
	// Check HKCU first (most common)
	hkcuPath := `Software\Classes\` + shellKeyName
	key, err := registry.OpenKey(registry.CURRENT_USER, hkcuPath, registry.READ)
	if err == nil {
		key.Close()
		return true
	}
	// Check HKCR (legacy / admin install)
	key, err = registry.OpenKey(registry.CLASSES_ROOT, shellKeyName, registry.READ)
	if err == nil {
		key.Close()
		return true
	}
	return false
}

// Register adds the cascading context menu entry with two sub-items.
// Always registers under HKCU (no admin required, always deletable).
func Register() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine exe path: %w", err)
	}

	hkcuPath := `Software\Classes\` + shellKeyName
	return registerUnder(registry.CURRENT_USER, hkcuPath, exePath)
}

func registerUnder(root registry.Key, keyPath, exePath string) error {
	menuLabel := i18n.T("ctx.menu_label")

	// Create parent key: cascading menu
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

	// Sub-item 1: Add video
	addLabel := i18n.T("ctx.add")
	addKeyPath := keyPath + `\shell\01add`
	addKey, _, err := registry.CreateKey(root, addKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer addKey.Close()
	if err := addKey.SetStringValue("", addLabel); err != nil {
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

	// Sub-item 2: Add video and start
	startLabel := i18n.T("ctx.add_start")
	startKeyPath := keyPath + `\shell\02start`
	startKey, _, err := registry.CreateKey(root, startKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer startKey.Close()
	if err := startKey.SetStringValue("", startLabel); err != nil {
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

// Unregister removes the context menu entry from both HKCU and HKCR.
func Unregister() error {
	hkcuPath := `Software\Classes\` + shellKeyName
	errCU := deleteKeyTree(registry.CURRENT_USER, hkcuPath)

	// Also try HKCR in case it was registered there previously (admin)
	errCR := deleteKeyTree(registry.CLASSES_ROOT, shellKeyName)

	if errCU != nil && errCR != nil {
		return fmt.Errorf("HKCU: %v; HKCR: %v", errCU, errCR)
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
