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

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// metricsIngestion holds Prometheus metrics for the ingestion subsystem.
type metricsIngestion struct {
	once sync.Once

	// Delta
	deltaAdded    prometheus.Counter
	deltaModified prometheus.Counter
	deltaDeleted  prometheus.Counter
	deltaRenamed  prometheus.Counter

	// Delta (post-filter)
	deltaFilteredAdded    prometheus.Counter
	deltaFilteredModified prometheus.Counter
	deltaFilteredDeleted  prometheus.Counter
	deltaFilteredRenamed  prometheus.Counter

	// Functions/Embeddings
	funcsAdded    prometheus.Counter
	funcsModified prometheus.Counter
	funcsRemoved  prometheus.Counter
	embedComputed prometheus.Counter
	embedSkipped  prometheus.Counter
	embedErrors   prometheus.Counter
	embedRetries  prometheus.Counter

	// Batches
	batchesSent prometheus.Counter

	// Defensive cleanups
	pathSweeps      prometheus.Counter
	edgesOnlySweeps prometheus.Counter

	// Durations
	deltaDuration prometheus.Histogram
	parseDuration prometheus.Histogram
	embedDuration prometheus.Histogram
	writeDuration prometheus.Histogram
	totalDuration prometheus.Histogram
}

var ingMetrics metricsIngestion

func (m *metricsIngestion) init() {
	m.once.Do(func() {
		m.deltaAdded = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_added_total", Help: "Archivos añadidos detectados por delta"})
		m.deltaModified = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_modified_total", Help: "Archivos modificados detectados por delta"})
		m.deltaDeleted = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_deleted_total", Help: "Archivos eliminados detectados por delta"})
		m.deltaRenamed = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_renamed_total", Help: "Renames detectados por delta"})

		m.deltaFilteredAdded = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_filtered_added_total", Help: "Archivos añadidos tras filtros"})
		m.deltaFilteredModified = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_filtered_modified_total", Help: "Archivos modificados tras filtros"})
		m.deltaFilteredDeleted = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_filtered_deleted_total", Help: "Archivos eliminados tras filtros"})
		m.deltaFilteredRenamed = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_delta_filtered_renamed_total", Help: "Renames tras filtros"})

		m.funcsAdded = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_functions_added_total", Help: "Funciones añadidas"})
		m.funcsModified = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_functions_modified_total", Help: "Funciones modificadas"})
		m.funcsRemoved = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_functions_removed_total", Help: "Funciones removidas"})

		m.embedComputed = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_embeddings_computed_total", Help: "Embeddings calculados"})
		m.embedSkipped = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_embeddings_skipped_total", Help: "Embeddings reutilizados/caché"})
		m.embedErrors = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_embeddings_errors_total", Help: "Errores de proveedor de embeddings"})
		m.embedRetries = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_embeddings_retries_total", Help: "Reintentos de embeddings"})

		m.batchesSent = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_batches_sent_total", Help: "Batches enviados a Primary"})

		m.pathSweeps = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_path_sweeps_total", Help: "Limpiezas defensivas por ruta (rm_*_by_*_path)"})
		m.edgesOnlySweeps = prometheus.NewCounter(prometheus.CounterOpts{Name: "cie_ing_edges_only_sweeps_total", Help: "Limpiezas de solo edges por ruta (modificados sin manifest)"})

		buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
		m.deltaDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "cie_ing_delta_seconds", Help: "Duración de detección de delta", Buckets: buckets})
		m.parseDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "cie_ing_parse_seconds", Help: "Duración de parseo", Buckets: buckets})
		m.embedDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "cie_ing_embed_seconds", Help: "Duración de embeddings", Buckets: buckets})
		m.writeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "cie_ing_write_seconds", Help: "Duración de escrituras", Buckets: buckets})
		m.totalDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "cie_ing_total_seconds", Help: "Duración total de la ejecución", Buckets: buckets})

		prometheus.MustRegister(
			m.deltaAdded, m.deltaModified, m.deltaDeleted, m.deltaRenamed,
			m.deltaFilteredAdded, m.deltaFilteredModified, m.deltaFilteredDeleted, m.deltaFilteredRenamed,
			m.funcsAdded, m.funcsModified, m.funcsRemoved,
			m.embedComputed, m.embedSkipped, m.embedErrors, m.embedRetries,
			m.batchesSent,
			m.pathSweeps, m.edgesOnlySweeps,
			m.deltaDuration, m.parseDuration, m.embedDuration, m.writeDuration, m.totalDuration,
		)
	})
}

// record helpers - used by pipeline for metrics tracking
func recordEmbedRetry() { ingMetrics.init(); ingMetrics.embedRetries.Inc() }
