# Project Bootstrap (F0.M1)

Este paquete implementa el registro de proyectos y bootstrap de la base de datos para CIE Primary Hub.

## Funcionalidad

El módulo F0.M1 proporciona una forma determinista e idempotente de registrar un `project_id` y inicializar su base de datos con las tablas del sistema.

## Uso

### Registrar e inicializar un proyecto nuevo

```go
import (
    "github.com/kraklabs/kraken/internal/cie/bootstrap"
    reg "github.com/kraklabs/kraken/internal/cie/registry"
)

// Cargar el registro
registry, err := reg.Load("/CIE-data")
if err != nil {
    log.Fatal(err)
}

// Bootstrap del proyecto (registro + inicialización de DB)
entry, err := bootstrap.BootstrapProject(
    registry,
    "/CIE-data",           // base path
    "mi-proyecto",          // project_id
    "rocksdb",              // backend CozoDB
    nil,                    // logger (opcional)
)
if err != nil {
    log.Fatal(err)
}

// El proyecto está listo para recibir ExecuteWrite
log.Printf("Proyecto registrado: %s (UUID: %s)", entry.ProjectID, entry.ProjectUUID)
```

### Validación de project_id

El `project_id` debe cumplir:
- **Longitud**: 1-200 caracteres
- **Charset**: `[a-zA-Z0-9._-]` (alfanumérico, puntos, guiones bajos, guiones)
- **Ejemplos válidos**: `my-project`, `project_123`, `org.project.name`
- **Ejemplos inválidos**: `project@name` (carácter @), `" "` (vacío), `a`*201 (demasiado largo)

### Idempotencia

Llamar `BootstrapProject` múltiples veces con el mismo `project_id` es seguro:
- Si el proyecto ya existe, retorna la entrada existente sin error
- El UUID del proyecto nunca cambia
- La inicialización de la DB es idempotente (las tablas se crean solo si no existen)

## Estructura de directorios creada

Después del bootstrap, se crea la siguiente estructura:

```
/CIE-data/
├── registry/
│   └── projects.json          # Registro de proyectos
└── projects/
    └── {project_uuid}/
        ├── live/              # Base de datos CozoDB (RocksDB)
        ├── snapshots/         # Snapshots del proyecto
        └── wal/               # Write-Ahead Log (opcional)
```

## Tablas del sistema

Al inicializar la base de datos, se crean automáticamente:

1. **`cie_replication_log`**: Log de replicación
   - `index: Int` (clave primaria)
   - `script: String`
   - `timestamp: Int`
   - `request_id: String`

2. **`cie_metadata`**: Metadatos del sistema
   - `key: String` (clave primaria)
   - `value: String`
   - Claves inicializadas:
     - `log_index`: "0" (baseline)
     - `last_snapshot_version`: "none" (o placeholder)
     - `schema_version`: versión del schema actual

## Garantías post-bootstrap

Después de un bootstrap exitoso:

✅ El proyecto está registrado en `projects.json`  
✅ Los directorios del proyecto existen  
✅ La base de datos CozoDB está inicializada en `live/`  
✅ Las tablas del sistema existen y están validadas  
✅ `log_index` está en 0 (baseline)  
✅ El Primary Hub puede aceptar `ExecuteWrite` para este proyecto  

## Integración con ProjectManager

En producción, el `ProjectManager` maneja la apertura y gestión de conexiones a la base de datos. El bootstrap solo inicializa la estructura inicial; las operaciones posteriores usan `ProjectManager.WithProjectWrite()`.

## Manejo de errores

- **project_id inválido**: Retorna error antes de crear cualquier estructura
- **Error al crear directorios**: Limpia y retorna error
- **Error al inicializar DB**: Cierra la DB y retorna error
- **Inconsistencias en tablas del sistema**: Retorna error (requiere reparación manual)

## Tests

Los tests cubren:
- Creación de proyecto nuevo
- Idempotencia (múltiples llamadas)
- Validación de `project_id`
- Verificación de tablas del sistema
- Verificación de metadatos inicializados

Ejecutar tests:
```bash
go test ./internal/cie/bootstrap/...
```





