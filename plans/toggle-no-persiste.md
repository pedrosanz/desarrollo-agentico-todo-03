# Feature: Persistir el estado al marcar una tarea como realizada

## Encargo y criterios de aceptación

Encargo literal: «la aplicación no guarda el estado cuando se marca una tarea como realizada».

**Requisito explícito**

- Al marcar (o desmarcar) una tarea como realizada desde la lista, el cambio se guarda en la base de datos.

**Inferencias.** No se piden de forma expresa; se toman como decisiones de diseño.

- I1. Se conserva la respuesta inmediata en la interfaz (la tarea cambia de aspecto sin recargar la página) que introdujo `5fdd6c3 feat(ui): improve task completion feedback`.
- I2. Si el guardado falla, la interfaz no debe quedarse mostrando un estado que no existe en el servidor.
- I3. Se mantiene la protección CSRF y la ruta `PUT /tasks/{task_id}/toggle` sin cambios.

**Criterios de aceptación**

1. Con JavaScript activo, pulsar «Completar» en una tarea pendiente y recargar la página muestra la tarea completada (`aria-pressed="true"`, clase `task-completed`).
2. Pulsar «Reabrir» en una tarea completada y recargar la muestra pendiente.
3. La petición que envía el navegador es `POST /tasks/{id}/toggle` con `_method=PUT` y `authenticity_token`, y el servidor responde 303.
4. Si el servidor responde con error o la red falla, la página se recarga mediante el envío normal del formulario y muestra el estado real.
5. Sin JavaScript, el formulario sigue funcionando como hasta ahora (envío normal con PUT por `_method`).
6. Los navegadores reciben el JS nuevo, no el antiguo en caché.

## Comportamiento actual y propuesta

**Actual** (evidencia en el repositorio):

- `templates/tasks/index.plush.html`: cada tarea tiene `<form action="/tasks/{id}/toggle" method="POST" class="task-toggle" data-task-toggle>` con los campos ocultos `authenticity_token` y `_method=PUT`.
- `actions/app.go`: la única ruta es `app.PUT("/tasks/{task_id}/toggle", TasksToggle)`; `csrf.New` protege las peticiones no seguras.
- `public/assets/app.js`: el manejador de `submit` hace `event.preventDefault()`, cambia la interfaz y termina con `fetch(form.action)`.
  - `fetch` sin opciones es un **GET** sin cuerpo: no lleva `_method` ni `authenticity_token`.
  - No existe ruta GET para `/tasks/{id}/toggle`: el servidor responde 404 y no guarda nada. La respuesta no se comprueba, así que la interfaz ya ha cambiado y parece que todo ha ido bien hasta recargar.
- `buffalo@v1.1.4/method_override.go`: `MethodOverride` convierte un POST en el método indicado por `req.FormValue("_method")`.
- `gobuffalo/middleware@v1.0.0/csrf/csrf.go`: el token se lee de la cabecera `X-CSRF-Token` o de `PostFormValue("authenticity_token")`.
- `templates/application.plush.html`: `<script src="/assets/app.js?v=2" defer>`.
- `TasksToggle` y sus tests (`Test_TasksToggle_CompletesAndReopens`, `_NotFound`, `_InvalidID`) ya validan la parte de servidor con PUT: el defecto está solo en el cliente.

**Propuesta**: cambio mínimo en el cliente.

- Enviar el propio formulario con `fetch(form.action, { method: "POST", body: new URLSearchParams(new FormData(form)), credentials: "same-origin" })`. Así el servidor recibe exactamente lo mismo que en el envío sin JavaScript (`_method=PUT`, `authenticity_token`) codificado como `application/x-www-form-urlencoded`.
- Comprobar `response.ok`; si no lo es o la promesa falla, llamar a `form.submit()`. `form.submit()` no dispara el evento `submit`, así que no hay bucle; navega con el envío normal y la página muestra el estado real del servidor.
- Extraer el código que actualiza la interfaz a una función `renderTaskCompleted(task, form, completed)` para leer el flujo con claridad.
- Subir `app.js?v=2` → `app.js?v=3`.

## Alcance

**Incluido**

- `public/assets/app.js`: petición correcta y recuperación ante error.
- `templates/application.plush.html`: versión del script.
- Test de acción que fija el contrato del formulario del que depende el JS.

**Excluido**

- Cambios en rutas, acciones, modelo o migraciones: el servidor ya funciona.
- Herramientas de test de JavaScript (no existen en el repositorio; añadirlas es un cambio de infraestructura aparte).
- Evitar condiciones de carrera por doble clic rápido: cada clic envía su propio toggle y la interfaz alterna en paralelo, así que el estado final coincide.
- Los tests de `todo/adws`, que fallan por llamar a GitHub real; no tienen relación con esta funcionalidad.

## Diseño por capas

### JavaScript (`public/assets/app.js`)

Tras `event.preventDefault()` en la rama `[data-task-toggle]`:

```js
var task = form.closest(".task");
renderTaskCompleted(task, form, !task.classList.contains("task-completed"));

// El formulario incluye _method=PUT y authenticity_token: se envía tal cual
// para que el servidor reciba lo mismo que en un envío sin JavaScript.
fetch(form.action, {
  method: "POST",
  body: new URLSearchParams(new FormData(form)),
  credentials: "same-origin",
})
  .then(function (response) {
    if (!response.ok) {
      throw new Error("HTTP " + response.status);
    }
  })
  .catch(function () {
    // Si no se ha guardado, el envío normal muestra el estado real.
    form.submit();
  });
```

`renderTaskCompleted(task, form, completed)` contiene el código actual de actualización de la interfaz (clase `task-completed` con `classList.toggle("task-completed", completed)`, `.toggle-label`, `aria-pressed`, `aria-label`, `title` y el `span.task-status`), sin cambios de comportamiento.

El 303 del servidor redirige a `/`; `fetch` lo sigue con un GET a `/` que devuelve 200, así que `response.ok` es `true` en el caso correcto.

### Renderizado (`templates/application.plush.html`)

- `app.js?v=2` → `app.js?v=3`, mismo patrón de versión que ya se usa para `app.css`.

### Tests (`actions/tasks_test.go`)

- `Test_TasksIndex_ToggleFormSendsPut`: con una tarea creada, `GET /` contiene, en el formulario del toggle, `data-task-toggle`, `name="_method" value="PUT"` y `name="authenticity_token"`. Fija el contrato del que depende `app.js`.
- `Test_TasksToggle_GetDoesNotChangeState`: `GET /tasks/{id}/toggle` responde 404 y la tarea sigue pendiente. Documenta por qué el cliente no puede usar GET.

## Archivos relevantes

| Ruta | Responsabilidad | Cambio |
|---|---|---|
| `public/assets/app.js` | Toggle con respuesta inmediata | Modificar |
| `templates/application.plush.html` | Carga del JS con versión | Modificar |
| `actions/tasks_test.go` | Tests del contrato del formulario y de la ruta | Modificar |
| `templates/tasks/index.plush.html` | Formulario del toggle | Sin cambios (referencia) |
| `actions/app.go`, `actions/tasks.go` | Ruta y acción `TasksToggle` | Sin cambios (referencia) |

## Implementación paso a paso

1. En `public/assets/app.js`, extraer `renderTaskCompleted` y sustituir `fetch(form.action)` por la petición POST con recuperación descrita en «Diseño por capas».
2. Comprobar sintaxis con `node --check public/assets/app.js`.
3. En `templates/application.plush.html`, subir `app.js?v=2` a `app.js?v=3`.
4. Añadir los dos tests a `actions/tasks_test.go`, siguiendo el estilo de `Test_TasksToggle_CompletesAndReopens`.
5. Ejecutar las validaciones.

## Tests y validación

**Requisitos previos:** PostgreSQL accesible con `database.yml` (entorno `test`, BD `todo_test`), `GO_ENV=test` y migraciones de test aplicadas.

**Comandos**

```
gofmt -l .                                  # sin salida
go vet ./...
node --check public/assets/app.js
GO_ENV=test go test -p 1 ./actions/ ./models/
go build ./...
git diff --check
```

Se limita `go test` a `./actions/` y `./models/` porque son los paquetes afectados; `./adws/` falla de antemano por depender de GitHub real y no guarda relación con este cambio.

**Comprobación manual** (el repositorio no tiene tests de JavaScript):

- Arrancar la app con el código de la rama, marcar una tarea, recargar y comprobar que sigue marcada; desmarcar y repetir.
- Opcional sin navegador: reproducir la petición del JS con curl (`POST` con `_method=PUT` y `authenticity_token`, cookie de sesión) y comprobar 303 y el estado tras recargar; un GET a la misma URL debe dar 404.

## Riesgos y recuperación

- **Sin test automático del JS.** Una regresión en `app.js` no la detecta la suite; los tests nuevos solo fijan el contrato del formulario y de la ruta. Mitigación: comprobación manual y, en el futuro, un test de navegador.
- **Caché del navegador.** Si no se sube la versión, los usuarios seguirían con el JS roto. Cubierto en el paso 3.
- **Recuperación con `form.submit()`.** Si la tarea se borró en otra pestaña, el envío normal mostrará la página 404 de la app. Es el estado real y es preferible a un cambio visual falso.
- **Recuperación.** El cambio es solo de cliente y una plantilla: revertir el commit restaura el comportamiento anterior sin migraciones ni datos afectados.
