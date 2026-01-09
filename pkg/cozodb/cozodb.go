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
// Package cozodb provides a Go binding for CozoDB v0.7.6+
//
// This is a custom binding that supports the v0.7.6 C API, which includes
// the `immutable_query` parameter in cozo_run_query. The official cozo-lib-go
// v0.7.5 does not support this parameter, causing undefined behavior with
// newer libcozo_c versions.
package cozodb

/*
#include <stdlib.h>
#include <string.h>
#include "cozo_c.h"

// CGO flags for linking - these are overridden by environment variables
#cgo LDFLAGS: -lcozo_c -lstdc++ -lm
#cgo windows LDFLAGS: -lbcrypt -lwsock32 -lws2_32 -lshlwapi -lrpcrt4
#cgo darwin LDFLAGS: -framework Security
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"unsafe"
)

// CozoDB represents an open CozoDB database instance.
type CozoDB struct {
	id     C.int32_t
	closed bool
}

// NamedRows represents the result of a query with column headers and data rows.
type NamedRows struct {
	Headers []string
	Rows    [][]any
}

// New opens a new CozoDB database.
//
// engine: storage engine to use - "mem", "sqlite", or "rocksdb"
// path: path to the database directory (ignored for "mem")
// options: engine-specific options as a map (can be nil)
func New(engine, path string, options map[string]any) (CozoDB, error) {
	cEngine := C.CString(engine)
	defer C.free(unsafe.Pointer(cEngine))

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// Convert options map to JSON string
	optionsJSON := "{}"
	if options != nil && len(options) > 0 {
		optBytes, err := json.Marshal(options)
		if err != nil {
			return CozoDB{}, fmt.Errorf("marshal options: %w", err)
		}
		optionsJSON = string(optBytes)
	}
	log.Printf("[COZO] Opening DB: engine=%s path=%s options=%s", engine, path, optionsJSON)
	cOptions := C.CString(optionsJSON)
	defer C.free(unsafe.Pointer(cOptions))

	var dbID C.int32_t
	errPtr := C.cozo_open_db(cEngine, cPath, cOptions, &dbID)

	if errPtr != nil {
		errMsg := C.GoString(errPtr)
		C.cozo_free_str(errPtr)
		return CozoDB{}, errors.New(errMsg)
	}

	return CozoDB{id: dbID}, nil
}

// Run executes a CozoScript query against the database.
//
// script: the CozoScript to execute
// params: query parameters as a map (can be nil)
//
// This method passes immutable_query=false to allow write operations.
func (db *CozoDB) Run(script string, params map[string]any) (NamedRows, error) {
	return db.runQuery(script, params, false)
}

// RunReadOnly executes a read-only CozoScript query against the database.
//
// script: the CozoScript to execute
// params: query parameters as a map (can be nil)
//
// This method passes immutable_query=true to enforce read-only semantics.
// Write operations will fail with an error.
func (db *CozoDB) RunReadOnly(script string, params map[string]any) (NamedRows, error) {
	return db.runQuery(script, params, true)
}

// runQuery is the internal implementation that calls the C API.
func (db *CozoDB) runQuery(script string, params map[string]any, immutable bool) (NamedRows, error) {
	if db.closed {
		return NamedRows{}, errors.New("database is closed")
	}

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	// Convert params map to JSON string
	paramsJSON := "{}"
	if params != nil && len(params) > 0 {
		paramBytes, err := json.Marshal(params)
		if err != nil {
			return NamedRows{}, fmt.Errorf("marshal params: %w", err)
		}
		paramsJSON = string(paramBytes)
	}
	cParams := C.CString(paramsJSON)
	defer C.free(unsafe.Pointer(cParams))

	// Call the C API with the immutable_query parameter
	cImmutable := C.bool(immutable)
	resultPtr := C.cozo_run_query(db.id, cScript, cParams, cImmutable)

	if resultPtr == nil {
		return NamedRows{}, errors.New("cozo_run_query returned null")
	}

	resultJSON := C.GoString(resultPtr)
	C.cozo_free_str(resultPtr)

	// Parse the JSON result
	return parseResult(resultJSON)
}

// Close closes the database connection.
func (db *CozoDB) Close() bool {
	if db.closed {
		return false
	}
	db.closed = true
	return bool(C.cozo_close_db(db.id))
}

// parseResult parses the JSON result from CozoDB into NamedRows.
func parseResult(jsonStr string) (NamedRows, error) {
	var result struct {
		OK      bool     `json:"ok"`
		Headers []string `json:"headers"`
		Rows    [][]any  `json:"rows"`
		Message string   `json:"message"`
		Display string   `json:"display"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return NamedRows{}, fmt.Errorf("parse result: %w", err)
	}

	if !result.OK {
		errMsg := result.Message
		if errMsg == "" {
			errMsg = result.Display
		}
		if errMsg == "" {
			errMsg = "query failed"
		}
		return NamedRows{}, errors.New(errMsg)
	}

	return NamedRows{
		Headers: result.Headers,
		Rows:    result.Rows,
	}, nil
}

// Backup creates a backup of the database to the specified path.
func (db *CozoDB) Backup(outPath string) error {
	if db.closed {
		return errors.New("database is closed")
	}

	cPath := C.CString(outPath)
	defer C.free(unsafe.Pointer(cPath))

	resultPtr := C.cozo_backup(db.id, cPath)
	if resultPtr == nil {
		return errors.New("cozo_backup returned null")
	}

	resultJSON := C.GoString(resultPtr)
	C.cozo_free_str(resultPtr)

	var result struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("parse backup result: %w", err)
	}
	if !result.OK {
		return errors.New(result.Message)
	}
	return nil
}

// Restore restores the database from a backup file.
func (db *CozoDB) Restore(inPath string) error {
	if db.closed {
		return errors.New("database is closed")
	}

	cPath := C.CString(inPath)
	defer C.free(unsafe.Pointer(cPath))

	resultPtr := C.cozo_restore(db.id, cPath)
	if resultPtr == nil {
		return errors.New("cozo_restore returned null")
	}

	resultJSON := C.GoString(resultPtr)
	C.cozo_free_str(resultPtr)

	var result struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("parse restore result: %w", err)
	}
	if !result.OK {
		return errors.New(result.Message)
	}
	return nil
}

// ImportRelations imports data into relations from a JSON payload.
func (db *CozoDB) ImportRelations(jsonPayload string) error {
	if db.closed {
		return errors.New("database is closed")
	}

	cPayload := C.CString(jsonPayload)
	defer C.free(unsafe.Pointer(cPayload))

	resultPtr := C.cozo_import_relations(db.id, cPayload)
	if resultPtr == nil {
		return errors.New("cozo_import_relations returned null")
	}

	resultJSON := C.GoString(resultPtr)
	C.cozo_free_str(resultPtr)

	var result struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("parse import result: %w", err)
	}
	if !result.OK {
		return errors.New(result.Message)
	}
	return nil
}

// ExportRelations exports relations to a JSON string.
func (db *CozoDB) ExportRelations(jsonPayload string) (string, error) {
	if db.closed {
		return "", errors.New("database is closed")
	}

	cPayload := C.CString(jsonPayload)
	defer C.free(unsafe.Pointer(cPayload))

	resultPtr := C.cozo_export_relations(db.id, cPayload)
	if resultPtr == nil {
		return "", errors.New("cozo_export_relations returned null")
	}

	resultJSON := C.GoString(resultPtr)
	C.cozo_free_str(resultPtr)

	return resultJSON, nil
}
