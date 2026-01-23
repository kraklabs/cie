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

// Package bootstrap handles CIE project initialization and setup.
//
// This internal package provides the core initialization logic for CIE projects.
// It creates CozoDB databases with the required schema for code indexing and
// ensures all prerequisites are met before the project can be used.
//
// # Initialization Workflow
//
// A typical workflow for setting up a new CIE project:
//
//	// Initialize the project (creates database and schema)
//	info, err := bootstrap.InitProject(bootstrap.ProjectConfig{
//	    ProjectID: "myproject",
//	    Engine:    "rocksdb",  // Optional: defaults to rocksdb
//	}, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Project initialized at: %s\n", info.DataDir)
//
//	// Later, open the project for queries
//	backend, err := bootstrap.OpenProject(bootstrap.ProjectConfig{
//	    ProjectID: "myproject",
//	}, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer backend.Close()
//
// # Idempotency
//
// The InitProject function is idempotent: calling it multiple times on the
// same project is safe and will not corrupt existing data. This makes it
// suitable for use in scripts and automated workflows.
//
// # Configuration
//
// ProjectConfig controls the initialization behavior:
//
//   - ProjectID: Required. Logical identifier for the project.
//   - DataDir: Optional. Where to store CozoDB data. Defaults to ~/.cie/data/<project_id>.
//   - Engine: Optional. CozoDB storage engine. One of "mem", "sqlite", "rocksdb".
//     Defaults to "rocksdb" for persistent storage.
//
// # Storage Engines
//
// CIE supports three CozoDB storage engines:
//
//   - rocksdb: Production-grade persistent storage (default, recommended)
//   - sqlite: Lightweight persistent storage for smaller projects
//   - mem: In-memory storage for testing and temporary use
//
// # Project Discovery
//
// List existing projects in the default data directory:
//
//	projects, err := bootstrap.ListProjects()
//	for _, id := range projects {
//	    fmt.Println(id)
//	}
package bootstrap
