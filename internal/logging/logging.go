// Package logging sets up a persistent debug log file in addition to the
// charmbracelet/log terminal logger used for user-facing output.
//
// Log location (in priority order):
//  1. config.log_path if set
//  2. $XDG_DATA_HOME/rrun/rrun.log
//  3. ~/.local/share/rrun/rrun.log
package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/natefinch/lumberjack.v2"
)

// File is the structured debug logger that writes to the log file.
// It is nil until Init is called.
var File *slog.Logger

// Init sets up the file logger. logPath may be empty to use the XDG default.
func Init(logPath string) error {
	if logPath == "" {
		logPath = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	w := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,  // MB per file before rotation
		MaxBackups: 3,   // keep 3 rotated files
		Compress:   true,
	}
	File = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return nil
}

// DefaultPath returns the default log file path for the current platform.
//
// Priority:
//   - macOS:   ~/Library/Logs/rrun/rrun.log
//   - Windows: %LOCALAPPDATA%/rrun/rrun.log
//   - Linux:   $XDG_DATA_HOME/rrun/rrun.log  (default: ~/.local/share/rrun/rrun.log)
func DefaultPath() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Logs", "rrun", "rrun.log")
		}
	case "windows":
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			return filepath.Join(appData, "rrun", "rrun.log")
		}
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "rrun", "rrun.log")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "rrun", "rrun.log")
}
