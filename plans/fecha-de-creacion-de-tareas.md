# Feature: Fecha de creación visible en las tareas

## Encargo y criterios de aceptación

Encargo literal: «quiero que en las tareas añadas un campo de la fecha de creacion».

**Requisito explícito**

- Cada tarea muestra su fecha de creación.

**Inferencias.** El encargo no las precisa; se toman como decisiones de diseño revisables en el PR.

- I1. El «campo» ya existe en persistencia (`tasks.created_at`, creado por `t.Timestamps()`, y `Task.CreatedAt` en el modelo). El cambio consiste en **mostrarlo**, no en añadir una columna nueva.
- I2. La fecha la asigna el sistema al crear la tarea y es **de solo lectura**: no aparece en el formulario de creación ni en el de edición, y no se acepta desde el cliente (`CreatedAt` ya tiene `form:"-"`).
- I3. Se muestra en la **lista** (`/`), en la misma línea que el título y la prioridad (entre ambos), con el texto «Creada el dd/mm/aaaa» (formato español, solo día, sin hora). La misma línea es un requisito pedido por el usuario al revisar la primera versión, que la colocaba bajo el título.
- I4. La fecha se formatea en la zona horaria en que la devuelve la base de datos (la columna es `timestamp` sin zona; en el contenedor y en CI es UTC). No se introduce conversión de zona horaria.
- I5. El orden de la lista no cambia (`created_at asc, id asc`).

**Criterios de aceptación**

1. En `/`, cada tarea muestra «Creada el dd/mm/aaaa» con su fecha de creación, dentro de un elemento `<time>` cuyo atributo `datetime` es la fecha en formato `aaaa-mm-dd`.
2. Una tarea creada el 5 de marzo de 2026 muestra exactamente `Creada el 05/03/2026` (día antes que mes).
3. Las tareas completadas también muestran la fecha, con el estilo atenuado del resto de la fila.
4. Editar una tarea no cambia su fecha de creación (ya cubierto por `Test_TasksUpdate_PersistsTitleOnly`).
5. No hay migraciones nuevas ni cambios en formularios.

## Comportamiento actual y propuesta

**Actual** (evidencia en el repositorio):

- `migrations/20260901000000_create_tasks.up.fizz`: `t.Timestamps()` crea `created_at` y `updated_at`.
- `models/task.go`: `Task.CreatedAt time.Time` con tags `db:"created_at" form:"-"`. Patrón de presentación existente: `func (t Task) PriorityLabel() string`.
- `templates/tasks/index.plush.html`: cada `<li class="task">` contiene, en orden, el formulario de toggle, `<span class="task-title">`, `<span class="task-priority …">`, el `<span class="task-status">` condicional (solo lectores de pantalla, posicionado absoluto) y `<div class="task-actions">`.
- `public/assets/app.css`: `.task` es un grid de 4 columnas (`44px minmax(0, 1fr) auto auto`; en móvil `36px …`), `align-items: start`. `.task-title` tiene `padding: 0.65rem 0`. Las filas completadas atenúan título y prioridad (`.task-completed .task-title`, `.task-completed .task-priority`).
- La fecha de creación no se muestra en ningún sitio.

**Propuesta**: cambio vertical mínimo en modelo, plantilla y CSS.

- Dos métodos de presentación en `Task`, siguiendo el patrón de `PriorityLabel()`: `CreatedDateLabel()` (`02/01/2006`) y `CreatedDateISO()` (`2006-01-02`).
- En la lista, un `<time class="task-created">` justo después del título.
- En CSS, el grid de `.task` pasa de 4 a 5 columnas para que el `<time>` ocupe su propia columna `auto` entre el título y la prioridad, en la misma fila.

## Alcance

**Incluido**

- Métodos `CreatedDateLabel()` y `CreatedDateISO()` en `models/task.go`.
- Marcado de la fecha en `templates/tasks/index.plush.html`.
- Estilos `.task-created` en `public/assets/app.css`.
- Tests de modelo y de acción.

**Excluido**

- Migraciones o cambios de esquema.
- Mostrar la fecha en el formulario de edición.
- Mostrar la hora, fechas relativas («hace 2 días») o conversión de zona horaria.
- Ordenar o filtrar por fecha.
- Mostrar `updated_at`.

## Diseño por capas

**Modelo** (`models/task.go`)

```go
// CreatedDateLabel devuelve la fecha de creación en formato dd/mm/aaaa.
func (t Task) CreatedDateLabel() string {
	return t.CreatedAt.Format("02/01/2006")
}

// CreatedDateISO devuelve la fecha de creación en formato aaaa-mm-dd, válido
// para el atributo datetime de <time>.
func (t Task) CreatedDateISO() string {
	return t.CreatedAt.Format("2006-01-02")
}
```

**Plantilla** (`templates/tasks/index.plush.html`), inmediatamente después de `<span class="task-title"><%= t.Title %></span>`:

```html
<time class="task-created" datetime="<%= t.CreatedDateISO() %>">Creada el <%= t.CreatedDateLabel() %></time>
```

**CSS** (`public/assets/app.css`), junto a las reglas de `.task-priority`:

```css
.task-created {
  align-self: center;
  color: var(--color-muted);
  font-size: 0.75rem;
  white-space: nowrap;
}
```

Y en `.task` (escritorio y la variante móvil), añadir una columna `auto`: `44px minmax(0, 1fr) auto auto auto` y `36px minmax(0, 1fr) auto auto auto`. El orden de la fila queda: toggle, título, fecha, prioridad, acciones. `.task-status` sigue fuera del flujo (posición absoluta), así que no ocupa columna. El título es la única columna flexible: si falta espacio, es el título el que se parte en varias líneas; la fecha no se parte (`white-space: nowrap`). En filas completadas el color ya es atenuado; no hace falta una regla extra.

## Archivos relevantes

| Ruta | Responsabilidad |
| --- | --- |
| `models/task.go` | Modelo `Task`; añadir `CreatedDateLabel()` y `CreatedDateISO()`. |
| `models/task_test.go` | Tests de modelo; añadir test de formato. |
| `templates/tasks/index.plush.html` | Lista de tareas; añadir el `<time>`. |
| `public/assets/app.css` | Estilos de la fila; añadir `.task-created` y una columna al grid de `.task`. |
| `templates/application.plush.html` | Layout; subir la versión del CSS (`app.css?v=4` → `?v=5`) para invalidar la caché, como en cambios de CSS anteriores. |
| `actions/tasks_test.go` | Tests de acción; añadir test de renderizado. |
| `migrations/20260901000000_create_tasks.up.fizz` | Solo referencia: confirma que `created_at` ya existe. |

## Implementación paso a paso

1. En `models/task.go`, añadir los dos métodos de la sección «Diseño por capas» después de `PriorityLabel()`. `time` ya está importado.
2. En `templates/tasks/index.plush.html`, añadir la línea `<time class="task-created" …>` justo después del `<span class="task-title">`.
3. En `public/assets/app.css`, añadir la regla `.task-created` después de las reglas de `.task-priority` (antes de `.task-completed .task-priority`) y añadir la quinta columna `auto` a `grid-template-columns` de `.task`, tanto en la regla general como en la de móvil.
4. En `templates/application.plush.html`, cambiar `/assets/app.css?v=4` por `/assets/app.css?v=5`.
5. Añadir los tests de la sección siguiente.
6. Ejecutar los comandos de validación.

## Tests y validación

**Requisitos previos:**

- PostgreSQL accesible según `database.yml` (entorno `test`, BD `todo_test`, `127.0.0.1:5432`; `compose.yaml` publica el puerto del servicio `db`).
- `GO_ENV=test`.
- Migraciones de test aplicadas (`soda migrate -e test`). Este cambio no añade migraciones.

**`models/task_test.go`** (nuevo)

- `Test_Task_CreatedDateLabels`: con `Task{CreatedAt: time.Date(2026, time.March, 5, 10, 30, 0, 0, time.UTC)}`, `CreatedDateLabel()` devuelve `"05/03/2026"` y `CreatedDateISO()` devuelve `"2026-03-05"`. No necesita base de datos.

**`actions/tasks_test.go`** (nuevo)

- `Test_TasksIndex_ShowsCreationDate`: crear una tarea con `as.DB.ValidateAndCreate`, fijar su fecha con `as.DB.RawQuery("UPDATE tasks SET created_at = ? WHERE id = ?", time.Date(2026, time.March, 5, 10, 30, 0, 0, time.UTC), task.ID).Exec()`, pedir `/` y comprobar que el body contiene `<time class="task-created" datetime="2026-03-05">Creada el 05/03/2026</time>`. Se usa SQL directo para no depender de si pop respeta un `CreatedAt` no nulo al crear.
- Los tests existentes deben pasar **sin cambios**, en especial `Test_TasksIndex_ListsTasks`, `Test_TasksIndex_ShowsPriorities` y `Test_TasksUpdate_PersistsTitleOnly`.

**Comandos**

```bash
gofmt -l .            # sin salida
go vet ./...
GO_ENV=test go test -p 1 ./...
go build ./...
```

## Riesgos y recuperación

- **Zona horaria (I4).** Una tarea creada cerca de medianoche puede mostrar el día UTC y no el local. Se acepta en esta iteración; si se necesita hora local, sería un cambio aparte (convertir con `time.LoadLocation` en los métodos del modelo).
- **Maquetación en pantallas estrechas.** Con fecha y prioridad en la misma línea, al título le queda menos ancho y en móvil se partirá antes en varias líneas. Recuperación: comprobarlo visualmente en escritorio y en móvil; si hiciera falta, acortar el texto visible en móvil sería un cambio aparte.
- **Test con SQL directo.** Si el `UPDATE` no se aplica (por ejemplo, por transacción), el test fallará con la fecha actual en el body; la causa es el test, no la plantilla.
- **Reversión.** El cambio no toca el esquema: basta con revertir el commit.
