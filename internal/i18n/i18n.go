package i18n

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// Lang represents a supported language.
type Lang int

const (
	LangJA Lang = iota
	LangEN
)

var current Lang

func init() {
	if isJapanese() {
		current = LangJA
	} else {
		current = LangEN
	}
}

func isJapanese() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultUILanguage")
	r, _, _ := proc.Call()
	langID := uint16(r)
	// Japanese primary language ID = 0x11
	primary := langID & 0x3FF
	return primary == 0x11
}

// Current returns the current language.
func Current() Lang { return current }

// SetLang overrides the current language.
func SetLang(l Lang) { current = l }

// T returns the localized string for the given key.
func T(key string) string {
	if s, ok := table[current][key]; ok {
		return s
	}
	// Fallback to English
	if s, ok := table[LangEN][key]; ok {
		return s
	}
	return key
}

// Tf returns a formatted localized string.
func Tf(key string, args ...interface{}) string {
	return fmt.Sprintf(T(key), args...)
}

// ---- OS language name for detection (exported for registry) ----

// GetUserDefaultUILanguage returns the Windows UI language LANGID.
func GetUserDefaultUILanguage() uint16 {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultUILanguage")
	r, _, _ := proc.Call()
	return uint16(r)
}

// ---- String table ----

var table = map[Lang]map[string]string{
	LangJA: {
		// Window
		"app.title": "EZ265 - H.265 一発変換",

		// Table columns
		"col.filename": "ファイル名",
		"col.origsize": "元サイズ",
		"col.status":   "状態",
		"col.saved":    "削減量",

		// Status
		"status.pending":   "待機中",
		"status.encoding":  "処理中...",
		"status.done":      "✓ 完了",
		"status.error":     "✗ エラー",
		"status.skipped":   "スキップ",
		"status.cancelled": "⊘ キャンセル",
		"status.encoding_pct": "%.0f%% 処理中...",

		// Size
		"size.increase": "+%s (増加)",
		"size.decrease": "-%s (%.0f%%)",
		"size.estimate": "推定:-%s",

		// Buttons
		"btn.add_files": "+ ファイルを追加",
		"btn.remove":    "削除",
		"btn.start":     "▶ スタート",
		"btn.pause":     "⏸ 一時停止",
		"btn.resume":    "▶ 再開",
		"btn.reg_add":   "右クリックメニュー登録",
		"btn.reg_del":   "右クリックメニュー削除",

		// Labels
		"lbl.rate":         "圧縮率: %d%%",
		"lbl.current_idle": "現在: (待機中)",
		"lbl.current_done": "現在: (完了)",
		"lbl.current_enc":  "現在: %s (%.0f%%)",
		"lbl.total":        "全体: (%d / %d 完了)",
		"lbl.total_done":   "全体: (%d / %d 完了) ✓",

		// Checkboxes
		"cb.append_h265": "h265追記",
		"cb.append_rate": "圧縮率追記",
		"cb.trash":       "元ファイルをゴミ箱へ",
		"cb.low_priority":"低優先度",
		"cb.shutdown":    "完了後シャットダウン",

		// Slider tooltip
		"slider.tooltip": "出力ビットレート = 元の映像ビットレート × 圧縮率",

		// File dialog
		"dlg.title":  "動画ファイルを選択",
		"dlg.filter": "動画ファイル",

		// Dialogs
		"dlg.confirm":         "確認",
		"dlg.error":           "エラー",
		"dlg.complete":        "完了",
		"dlg.cancel_encode":   "「%s」のエンコードをキャンセルしますか？",
		"dlg.remove_pending":  "「%s」をキューから削除しますか？",
		"dlg.force_quit":      "変換中です。強制終了しますか？",
		"dlg.reg_confirm_del": "右クリックメニューの登録を削除しますか？",
		"dlg.reg_failed":      "登録に失敗しました: %s",
		"dlg.reg_done":        "右クリックメニューに登録しました。",

		// Encoder status
		"enc.nvenc": "NVENC 有効 ✓ (%s)",
		"enc.cpu":   "CPU モード (x265)",

		// Menu
		"menu.settings":  "設定",
		"menu.language":  "言語",
		"menu.lang_ja":   "日本語",
		"menu.lang_en":   "English",
		"menu.help":      "ヘルプ",
		"menu.about":     "このソフトについて",

		// Context menu
		"ctx.menu_label": "EZ265",
		"ctx.add":        "動画を追加",
		"ctx.add_start":  "動画を追加してスタート",

		// About dialog
		"about.title": "H.265 一発変換 について",
		"about.text": "H.265 一発変換  v1.0\n\n" +
			"動画ファイルを H.265 (HEVC) 形式に一括変換するツールです。\n" +
			"NVIDIA NVENC による GPU アクセラレーション対応。\n" +
			"CPU (x265) フォールバックも搭載しています。\n\n" +
			"開発: ServalC4t\n" +
			"プロジェクトサイト: https://github.com/ServalC4t\n\n" +
			"--- 免責事項 ---\n" +
			"本ソフトウェアは「現状のまま」提供されます。\n" +
			"使用により生じたいかなる損害についても、\n" +
			"作者は一切の責任を負いません。\n" +
			"変換前のファイルのバックアップを推奨します。\n" +
			"本ソフトウェアは ffmpeg を利用しています。",
		"dlg.lang_restart": "言語を変更しました。再起動後に反映されます。",
	},
	LangEN: {
		// Window
		"app.title": "EZ265 - H.265 Converter",

		// Table columns
		"col.filename": "File Name",
		"col.origsize": "Original",
		"col.status":   "Status",
		"col.saved":    "Saved",

		// Status
		"status.pending":   "Pending",
		"status.encoding":  "Encoding...",
		"status.done":      "✓ Done",
		"status.error":     "✗ Error",
		"status.skipped":   "Skipped",
		"status.cancelled": "⊘ Cancelled",
		"status.encoding_pct": "%.0f%% Encoding...",

		// Size
		"size.increase": "+%s (increase)",
		"size.decrease": "-%s (%.0f%%)",
		"size.estimate": "Est:-%s",

		// Buttons
		"btn.add_files": "+ Add Files",
		"btn.remove":    "Remove",
		"btn.start":     "▶ Start",
		"btn.pause":     "⏸ Pause",
		"btn.resume":    "▶ Resume",
		"btn.reg_add":   "Add Context Menu",
		"btn.reg_del":   "Remove Context Menu",

		// Labels
		"lbl.rate":         "Rate: %d%%",
		"lbl.current_idle": "Current: (idle)",
		"lbl.current_done": "Current: (done)",
		"lbl.current_enc":  "Current: %s (%.0f%%)",
		"lbl.total":        "Total: (%d / %d done)",
		"lbl.total_done":   "Total: (%d / %d done) ✓",

		// Checkboxes
		"cb.append_h265": "Append h265",
		"cb.append_rate": "Append rate",
		"cb.trash":       "Trash original",
		"cb.low_priority":"Low priority",
		"cb.shutdown":    "Shutdown on done",

		// Slider tooltip
		"slider.tooltip": "Output bitrate = Original video bitrate × Rate",

		// File dialog
		"dlg.title":  "Select Video Files",
		"dlg.filter": "Video Files",

		// Dialogs
		"dlg.confirm":         "Confirm",
		"dlg.error":           "Error",
		"dlg.complete":        "Done",
		"dlg.cancel_encode":   "Cancel encoding \"%s\"?",
		"dlg.remove_pending":  "Remove \"%s\" from queue?",
		"dlg.force_quit":      "Encoding in progress. Force quit?",
		"dlg.reg_confirm_del": "Remove context menu entry?",
		"dlg.reg_failed":      "Registration failed: %s",
		"dlg.reg_done":        "Context menu registered.",

		// Encoder status
		"enc.nvenc": "NVENC Active ✓ (%s)",
		"enc.cpu":   "CPU Mode (x265)",

		// Menu
		"menu.settings":  "Settings",
		"menu.language":  "Language",
		"menu.lang_ja":   "日本語",
		"menu.lang_en":   "English",
		"menu.help":      "Help",
		"menu.about":     "About",

		// Context menu
		"ctx.menu_label": "EZ265",
		"ctx.add":        "Add video",
		"ctx.add_start":  "Add video & Start",

		// About dialog
		"about.title": "About H.265 Converter",
		"about.text": "H.265 Converter  v1.0\n\n" +
			"Batch convert video files to H.265 (HEVC) format.\n" +
			"Supports NVIDIA NVENC GPU acceleration.\n" +
			"Falls back to CPU (x265) when GPU is unavailable.\n\n" +
			"Developer: ServalC4t\n" +
			"Project: https://github.com/ServalC4t\n\n" +
			"--- Disclaimer ---\n" +
			"This software is provided \"as is\" without warranty\n" +
			"of any kind. The author is not responsible for any\n" +
			"damage caused by the use of this software.\n" +
			"Backing up original files is recommended.\n" +
			"This software uses ffmpeg.",
		"dlg.lang_restart": "Language changed. Restart to apply.",
	},
}

// Unused but ensures the linker doesn't strip the function.
var _ = unsafe.Pointer(nil)
var _ = strings.TrimSpace
