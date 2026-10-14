# Feature: Prioridad de tareas (alta, media, baja)

## Encargo y criterios de aceptación

Encargo literal: «añadir prioridades en base a 3 niveles, alta, media, baja».

**Requisitos explícitos**

- Cada tarea tiene una prioridad con uno de estos tres niveles: alta, media o baja.

**Inferencias.** El encargo no las pide; se toman como decisiones de diseño y se pueden revisar antes de `/implement`.

- I1. La prioridad se elige al crear la tarea y se puede cambiar en «Editar tarea».
- I2. La prioridad por defecto es **media**: para las tareas nuevas en las que no se elige y para todas las tareas existentes al migrar.
- I3. La prioridad se ve en la lista como etiqueta de texto («Alta», «Media», «Baja»). El color refuerza el texto, pero nunca es el único indicador.
- I4. El orden de la lista **no cambia**: sigue siendo `created_at asc, id asc`.

**Criterios de aceptación**

1. Tras la migración, todas las tareas existentes tienen prioridad `medium`. La columna es `NOT NULL`, con valor por defecto `'medium'`.
2. El formulario de nueva tarea (`/`) tiene un selector «Prioridad» con Alta, Media y Baja. Media aparece preseleccionada.
3. Si se crea una tarea con prioridad Alta, se guarda `high` y la lista muestra «Alta» en esa tarea.
4. Si se crea una tarea sin enviar `priority` (por ejemplo, un cliente que no conoce el campo), se guarda `medium`.
5. Si se envía un valor fuera de `high|medium|low` en la creación o en la edición, la respuesta es 422 con el mensaje «La prioridad no es válida.» y no se crea ni se modifica nada.
6. El formulario de edición muestra la prioridad actual preseleccionada. Al guardar con otra prioridad, esta se persiste y se conservan el título editado, `completed`, `id` y `created_at`.
7. Si en la edición no se envía `priority`, se mantiene la prioridad actual (no se reinicia a media).
8. Cuando la creación o la edición fallan por un título inválido, el formulario vuelve a mostrarse con la prioridad elegida por el usuario.
9. Cada tarea de la lista expone su prioridad a lectores de pantalla («Prioridad alta», etc.).

## Comportamiento actual y propuesta

**Actual** (evidencia en el repositorio):

- `migrations/20260901000000_create_tasks.up.fizz`: la tabla `tasks` tiene solo `id`, `title`, `completed` y timestamps.
- `models/task.go`: `Task` con `Title` (el único campo con `form:"title"`) y `Completed` (`form:"-"`). `Validate` solo valida el título.
- `actions/tasks.go`:
  - `TasksCreate` hace `c.Bind(task)` sobre el modelo.
  - `TasksUpdate` hace bind a un struct anónimo con **solo** `Title`, de forma deliberada para que no se puedan cambiar otros campos. El test `Test_TasksUpdate_PersistsTitleOnly` lo protege.
- `templates/tasks/index.plush.html`:
  - Cada `<li class="task">` es un grid de 3 columnas: toggle, título y acciones (`public/assets/app.css`, `.task`).
  - `.task-status` es solo para lectores de pantalla (comparte la regla de `.sr-only`).
- `templates/tasks/edit.plush.html`:
  - Solo edita el título.
  - La pista dice «Los demás datos de la tarea no cambian».
  - `aria-invalid`/`aria-describedby` del título dependen de `errors.HasAny()`, no de que haya errores en `title`.
- No hay estilos para `<select>` en `app.css`.

**Propuesta**: es un cambio vertical mínimo que sigue los patrones existentes.

- Nueva columna `priority` (texto con códigos estables en inglés: `high`/`medium`/`low`) con `CHECK` en base de datos.
- El modelo añade el campo, las constantes, la validación, un valor por defecto en `BeforeSave` y un método `PriorityLabel()` con la etiqueta en español.
- Un parcial `tasks/_priority_select.plush.html` se reutiliza en los dos formularios. Las opciones llegan con un helper de plantilla registrado en `actions/render.go`.
- En la lista, una etiqueta visible por tarea en una columna nueva del grid.

## Alcance

**Incluido**

- Migración que añade la columna `priority` y su restricción.
- Modelo: campo, constantes, validación, valor por defecto y etiqueta.
- Creación y edición con prioridad.
- Visualización de la prioridad en la lista.
- Estilos para el selector y la etiqueta.
- Fixture y tests de modelo y de acciones.

**Excluido**

- Ordenar o filtrar la lista por prioridad (ver «Alternativas» en Riesgos).
- Cambiar la prioridad directamente desde la lista, sin pasar por «Editar».
- Cambios en `app.js`.
- API JSON. El tag `json:"priority"` se añade por coherencia, pero no hay endpoints JSON.
- Corregir el defecto previo detectado en `public/assets/app.js`, que se describe en Riesgos. No forma parte de este encargo.

## Diseño por capas

### Persistencia y migración

Nueva migración `migrations/20261014000000_add_priority_to_tasks.up.fizz`:

```
add_column("tasks", "priority", "string", {"size": 10, "default": "medium"})
sql("ALTER TABLE tasks ADD CONSTRAINT tasks_priority_check CHECK (priority IN ('high', 'medium', 'low'))")
```

- En fizz las columnas son `NOT NULL` salvo que se indique `null: true`. Con `default`, PostgreSQL rellena las filas existentes con `'medium'`.
- `migrations/embed.go` ya embebe `*.fizz`, así que no hay que tocarlo.

Down (`...down.fizz`):

```
drop_column("tasks", "priority")
```

PostgreSQL elimina la restricción `CHECK` de una sola columna junto con la columna.

**No hay que modificar** la migración de seed `20260901000001_seed_tasks.up.sql`: sus `INSERT` no indican `priority` y toman el valor por defecto. No se editan migraciones ya aplicadas.

### Modelo (`models/task.go`)

```go
// Prioridades permitidas para una tarea.
const (
	PriorityHigh   = "high"
	PriorityMedium = "medium"
	PriorityLow    = "low"
)

// PriorityOption es una opción del selector de prioridad.
type PriorityOption struct {
	Value string
	Label string
}

// PriorityOptions devuelve las prioridades en orden de mayor a menor.
func PriorityOptions() []PriorityOption { ... } // high/Alta, medium/Media, low/Baja
```

- Campo nuevo: `Priority string `json:"priority" db:"priority" form:"priority"``.
- `Validate`: si `t.Priority != ""` y no es uno de los tres valores, `verrs.Add("priority", "La prioridad no es válida.")`. El valor vacío se acepta porque lo normaliza el paso siguiente.
- `func (t *Task) BeforeSave(tx *pop.Connection) error`: si `t.Priority == ""`, asigna `PriorityMedium`.
  - Así la creación vía `as.DB.ValidateAndCreate(&models.Task{Title: …})` sigue funcionando en los tests existentes y en `TasksToggle` (`tx.Update`).
  - Por qué no se exige `priority` en `Validate`: rompería todos los tests y llamadas que crean tareas sin prioridad.
- `func (t Task) PriorityLabel() string`: devuelve «Alta», «Media» o «Baja». Si el valor es desconocido, devuelve la cadena tal cual.
  - El receptor es por valor porque la plantilla itera `Tasks` (`[]Task`).

### Acción (`actions/tasks.go`)

- `TasksIndex`: `c.Set("task", &models.Task{Priority: models.PriorityMedium})` para que Media salga preseleccionada.
- `TasksCreate`: no cambia la lógica. `c.Bind(task)` recoge `priority` gracias al tag `form`. Ante errores ya vuelve a mostrar `task` con la prioridad elegida.
- `TasksUpdate`: añadir `Priority string `form:"priority" json:"priority"`` al struct anónimo. Tras asignar el título:

  ```go
  if input.Priority != "" {
  	task.Priority = input.Priority
  }
  ```

  - Se mantiene la protección de `completed`, `id` y `created_at`.
  - Actualizar el comentario de la función: «actualiza el título y la prioridad, conservando los demás datos de la tarea».
- No hay rutas nuevas (`actions/app.go` no cambia).

### Helper de plantilla (`actions/render.go`)

- Registrar en `Helpers`: `"priorityOptions": models.PriorityOptions`.
- Importar `todo/models`. No crea un ciclo, porque `actions` ya importa `models`.

### Renderizado (templates Plush)

Parcial nuevo `templates/tasks/_priority_select.plush.html`. Se invoca con `partial("tasks/priority_select.plush.html")`, igual que `partial("flash.plush.html")` resuelve `_flash.plush.html`.

```html
<label for="task-priority" class="priority-label">Prioridad</label>
<select
  id="task-priority"
  name="priority"
  <%= if (len(errors.Get("priority")) > 0) { %>aria-invalid="true" aria-describedby="priority-error"<% } %>
>
  <%= for (opt) in priorityOptions() { %>
    <option value="<%= opt.Value %>"<%= if (task.Priority == opt.Value) { %> selected<% } %>><%= opt.Label %></option>
  <% } %>
</select>
<%= if (len(errors.Get("priority")) > 0) { %>
  <p id="priority-error" class="field-error" role="alert">
    <%= for (msg) in errors.Get("priority") { %><%= msg %><% } %>
  </p>
<% } %>
```

`templates/tasks/index.plush.html`

- Formulario nuevo:
  - Incluir el parcial dentro de `.new-task-row`, entre el input y el botón «Añadir».
  - En el nuevo formulario la etiqueta «Prioridad» lleva `class="sr-only"`, igual que «Nueva tarea». Pasar una variable al parcial (`partial("…", {compact: true})`) o dejar que el CSS de `.new-task .priority-label` la oculte. Se elige el CSS para no complicar el parcial.
  - Cambiar las condiciones del input de título: `errors.HasAny()` → `len(errors.Get("title")) > 0`, tanto en el atributo como en el bloque `title-error`. Así un error de prioridad no marca el título como inválido.
- Lista: tras `<span class="task-title">…</span>`, añadir:

  ```html
  <span class="task-priority task-priority-<%= t.Priority %>"><span class="sr-only">Prioridad </span><%= t.PriorityLabel() %></span>
  ```

  - Queda **fuera** de `.task-title`, porque `app.js` usa `.task-title.textContent` para el `aria-label` del toggle.

`templates/tasks/edit.plush.html`

- Incluir el parcial después de la pista o del error del título y antes de `.edit-task-actions`.
- Cambiar la pista a «Hasta 200 caracteres.», porque «Los demás datos de la tarea no cambian» deja de ser exacto.
- Cambiar las condiciones de error del título a `len(errors.Get("title")) > 0`, igual que en el índice.

### CSS (`public/assets/app.css`) y caché

- Añadir `select` a la regla de `input[type="text"]` (o crear una regla equivalente) y a `input:focus-visible`, `input[aria-invalid="true"]`.
- `.edit-task .priority-label`: igual que `.edit-task label`, con `margin-top` para separarlo del bloque del título.
- `.new-task .priority-label`: ocultación accesible (copiar las propiedades de `.sr-only` o añadir el selector a esa regla).
- `.new-task-row select`: `width: auto; flex-shrink: 0`.
- `.task`: `grid-template-columns: 44px minmax(0, 1fr) auto auto` y, en `@media (max-width: 760px)`, `36px minmax(0, 1fr) auto auto`.
  - Aunque `.task-status` es `position: absolute` y no ocupa celda, la etiqueta de prioridad sí la ocupa.
- `.task-priority`: `align-self: center`, píldora pequeña (`font-size: 0.75rem`, `padding`, `border-radius`, `white-space: nowrap`).
- Variantes `-high` (tonos de `--color-primary`), `-medium` (neutro) y `-low` (apagado). El contraste del texto debe ser ≥ 4.5:1.
- `.task-completed .task-priority`: color apagado, para ser coherente con el título tachado.
- En `templates/application.plush.html`, subir `app.css?v=3` → `app.css?v=4` para invalidar la caché del navegador. Es el mismo patrón de versión que ya se usa.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
|---|---|---|
| `migrations/20261014000000_add_priority_to_tasks.up.fizz` | Añadir la columna y el `CHECK` | Nuevo |
| `migrations/20261014000000_add_priority_to_tasks.down.fizz` | Revertir | Nuevo |
| `models/task.go` | Campo, constantes, opciones, validación, `BeforeSave`, `PriorityLabel` | Modificar |
| `models/task_test.go` | Tests de modelo | Modificar |
| `actions/tasks.go` | Valor por defecto en el índice y bind de la prioridad en la edición | Modificar |
| `actions/render.go` | Helper `priorityOptions` | Modificar |
| `actions/tasks_test.go` | Tests de acciones | Modificar |
| `templates/tasks/_priority_select.plush.html` | Selector reutilizable | Nuevo |
| `templates/tasks/index.plush.html` | Selector en la creación y etiqueta en la lista | Modificar |
| `templates/tasks/edit.plush.html` | Selector en la edición y pista | Modificar |
| `templates/application.plush.html` | Cache-busting del CSS | Modificar |
| `public/assets/app.css` | Estilos del selector, la etiqueta y el grid | Modificar |
| `fixtures/tasks.toml` | Prioridades explícitas en el escenario «tres tareas» | Modificar |
| `migrations/embed.go`, `templates/embed.go`, `actions/app.go`, `public/assets/app.js`, seed SQL | Sin cambios (ya cubren lo necesario) | — |

## Implementación paso a paso

1. **Migración.** Crear los ficheros `up` y `down` con el contenido de «Persistencia y migración».
   - Aplicar con `soda migrate -e test` y, en desarrollo, arrancando la app (`cmd/app/main.go` llama a `models.Migrate()`).
   - Comprobar el rollback con `soda migrate down -e test` y volver a migrar.
2. **Modelo.** Añadir constantes, `PriorityOption`, `PriorityOptions()`, el campo `Priority`, la validación, `BeforeSave` y `PriorityLabel()` en `models/task.go`.
3. **Tests de modelo** (ver la sección siguiente). Ejecutarlos.
4. **Helper.** Registrar `priorityOptions` en `actions/render.go`.
5. **Acciones.** `TasksIndex` con el valor por defecto y `TasksUpdate` con el bind de `Priority`.
6. **Parcial y plantillas.** Crear `_priority_select.plush.html` y modificar `index` y `edit`, incluidas las condiciones de error del título.
7. **Fixture.** En `fixtures/tasks.toml`, añadir `priority = "high"`, `"medium"` y `"low"` a las tres filas, en ese orden.
8. **CSS y versión.** Aplicar los estilos y subir `?v=4`.
9. **Tests de acciones.** Añadirlos y ejecutar toda la batería.
10. **Revisión manual.** Arrancar la app (`/env:start`) y comprobar:
    - Crear con cada prioridad.
    - Editar la prioridad.
    - Error de título conservando la prioridad.
    - Vista móvil (≤ 760 px) sin desbordamiento.
    - Navegación con teclado por el selector.

## Tests y validación

**Requisitos previos:**
- PostgreSQL accesible con `database.yml` (entorno `test`, BD `todo_test`).
- `GO_ENV=test`.
- Migraciones aplicadas con `soda migrate -e test`, que incluyen la nueva.

Sin la migración, todos los tests de la suite fallarán por la columna inexistente.

**`models/task_test.go`** (nuevos)

- `Test_Task_Priority_DefaultsToMedium`: crear sin prioridad, recargar y comprobar `PriorityMedium`.
- `Test_Task_Priority_AcceptsAllLevels`: crear con `high`, `medium` y `low`, sin errores; tras recargar, el valor se conserva.
- `Test_Task_Priority_Invalid`: con `"urgente"`, `verrs.Get("priority")` contiene «La prioridad no es válida.» y el recuento es 0.
- `Test_Task_PriorityLabel`: comprobar las tres etiquetas.

**`actions/tasks_test.go`** (nuevos)

- `Test_TasksIndex_NewTaskDefaultsToMedium`: el body contiene `<option value="medium" selected>`. Ajustar la aserción al HTML real que genere el parcial.
- `Test_TasksIndex_ShowsPriorities`: con el fixture «tres tareas», el body contiene «Alta», «Media», «Baja» y `task-priority-high`.
- `Test_TasksCreate_WithPriority`: un POST con `title` y `priority=high` devuelve 303 y la BD tiene `high`. Usar `url.Values` porque `Post(&models.Task{…, Priority: "high"})` también sirve, ya que el campo tiene tag `form`.
- `Test_TasksCreate_WithoutPriority_DefaultsToMedium`: POST con solo `title`. Persiste `medium`.
- `Test_TasksCreate_InvalidPriority`: devuelve 422, el body contiene «La prioridad no es válida.» y el recuento es 0.
- `Test_TasksCreate_EmptyTitle_KeepsPriority`: título vacío y `priority=low` devuelven 422, y el body tiene `low` seleccionado.
- `Test_TasksEdit_PrefillsPriority`: una tarea `high` tiene `high` seleccionado en `/tasks/{id}/edit`.
- `Test_TasksUpdate_ChangesPriority`: un PUT con `title` y `priority=low` devuelve 303 y persiste `low`; `Completed` no cambia.
- `Test_TasksUpdate_WithoutPriority_KeepsCurrent`: una tarea `high` sigue en `high` tras un PUT con solo `title`.
- `Test_TasksUpdate_InvalidPriority`: devuelve 422 y la tarea no cambia (ni título ni prioridad).
- Los tests existentes deben pasar **sin cambios**. En especial, `Test_TasksUpdate_PersistsTitleOnly` (que envía solo `title`), `Test_TasksUpdate_EmptyTitle` (que comprueba `aria-describedby="title-error"`) y `Test_TasksIndex_ListsTasks`.

**Comandos**

```
gofmt -l .                 # sin salida
go vet ./...
soda migrate -e test
go test -p 1 ./...
go build ./...
```

## Riesgos y recuperación

- **Validez de dos supuestos de API sin verificar.** Al planificar no había `go` disponible en el entorno, así que no se pudo consultar el código de las dependencias. Estoy haciendo una inferencia en ambos casos, a partir de pop v6.4.1 y fizz:
  - (a) `ValidateAndCreate`/`ValidateAndUpdate` ejecutan `Validate` **antes** de `BeforeSave`, y `BeforeSave` se ejecuta también en `tx.Update`.
  - (b) fizz admite `sql("…")` en migraciones `.fizz`.

  Si (a) falla, porque `BeforeSave` no se dispara o se dispara antes de validar, el valor por defecto se pone explícitamente en `TasksCreate` y la validación sigue aceptando `""`. Si (b) falla, se usa una migración `.sql` par (`up.sql`/`down.sql`) con el `ALTER TABLE … ADD COLUMN … DEFAULT 'medium' NOT NULL` y el `CHECK`, ya que `embed.go` también embebe `*.sql`.
- **Plush y métodos/helpers.** `t.PriorityLabel()` y `priorityOptions()` dependen de que Plush invoque métodos de valor y funciones de helper sin argumentos. Si no lo hace, hay dos salidas: exponer la etiqueta como helper `priorityLabel(t.Priority)` o pasar las opciones con `c.Set` en las cuatro acciones que renderizan formularios.
- **Valor por defecto duplicado (BD y Go).** El valor `medium` vive en la migración y en `BeforeSave`. Si cambia uno sin el otro, divergen. Es aceptable por ahora; se documenta en la constante.
- **El `CHECK` frente a la validación de Go.** Si alguien amplía los niveles en Go sin migrar, la BD rechazará el `INSERT` con un error 500. Cualquier nivel nuevo exige una migración.
- **Grid de la lista.** La columna extra puede estrechar el título en móvil. Hay que revisar a 360-380 px. Si no cabe, la etiqueta pasa a mostrarse debajo del título (`grid-row`) solo en móvil.
- **Defecto previo, fuera de alcance.** En `public/assets/app.js`, el toggle hace `event.preventDefault()` y luego `fetch(form.action)`. Eso es un **GET** a `/tasks/{id}/toggle`, ruta que solo existe como PUT, y sin token CSRF. Estoy haciendo una inferencia: con JavaScript activo, completar una tarea solo cambia la UI y no se persiste. No afecta a esta feature; conviene abrir un `/bug` aparte.
- **Recuperación.**
  - Hacer `soda migrate down` (elimina la columna y el `CHECK`) y revertir el commit.
  - No se pierden datos previos. Solo se pierde la prioridad asignada desde el despliegue.

**Alternativas consideradas**

- *Entero 1-3* en lugar de texto: facilita ordenar por prioridad, pero es menos legible en la BD. El texto con `CHECK` permite ordenar en el futuro con `ORDER BY CASE priority …`.
- *Ordenar la lista por prioridad*: es la extensión natural, pero cambia el comportamiento actual y el encargo no la pide.
- *Ocultar la etiqueta «Media»* para reducir ruido visual: es más limpio, pero hace la información implícita. Se muestran los tres niveles.
