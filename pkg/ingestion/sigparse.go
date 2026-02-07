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

package ingestion

import "github.com/kraklabs/cie/pkg/sigparse"

// ParamInfo holds a parsed parameter's name and base type.
// Re-exported from pkg/sigparse for convenience.
type ParamInfo = sigparse.ParamInfo

// ParseGoSignatureParams parses a Go function signature string and returns
// the parameter names and their base types. Delegates to sigparse.ParseGoParams.
func ParseGoSignatureParams(signature string) []ParamInfo {
	return sigparse.ParseGoParams(signature)
}
