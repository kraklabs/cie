// Copyright 2025 KrakLabs
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
//
// For commercial licensing, contact: licensing@kraklabs.com
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
)

// ProgressConfig determines if and how progress should be displayed.
type ProgressConfig struct {
	// Enabled indicates whether progress bars should be shown.
	// Disabled when --json, -q flags are used, or when stderr is not a TTY.
	Enabled bool

	// Writer is where progress output goes (always os.Stderr).
	Writer io.Writer

	// NoColor disables colored output in progress bars.
	NoColor bool
}

// NewProgressConfig creates a progress configuration based on global flags and TTY detection.
//
// Progress is disabled when:
//   - --json flag is set (quiet is auto-set)
//   - -q/--quiet flag is set
//   - stderr is not a TTY (piped output, CI environments, etc.)
func NewProgressConfig(globals GlobalFlags) ProgressConfig {
	enabled := !globals.Quiet && isatty.IsTerminal(os.Stderr.Fd())

	return ProgressConfig{
		Enabled: enabled,
		Writer:  os.Stderr,
		NoColor: globals.NoColor,
	}
}

// NewProgressBar creates a progress bar with consistent styling.
// Returns nil if progress is disabled, allowing callers to safely check for nil.
//
// Parameters:
//   - cfg: Progress configuration from NewProgressConfig
//   - total: Total number of items to process
//   - description: Short description shown before the progress bar
func NewProgressBar(cfg ProgressConfig, total int64, description string) *progressbar.ProgressBar {
	if !cfg.Enabled {
		return nil
	}

	return progressbar.NewOptions64(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(cfg.Writer),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionEnableColorCodes(!cfg.NoColor),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

// NewSpinner creates an indeterminate progress spinner for operations
// where the total count is unknown.
// Returns nil if progress is disabled.
//
// Parameters:
//   - cfg: Progress configuration from NewProgressConfig
//   - description: Short description shown before the spinner
func NewSpinner(cfg ProgressConfig, description string) *progressbar.ProgressBar {
	if !cfg.Enabled {
		return nil
	}

	return progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(cfg.Writer),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionEnableColorCodes(!cfg.NoColor),
	)
}
