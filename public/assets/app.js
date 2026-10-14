// Pide confirmación antes de enviar formularios marcados con data-confirm.
document.addEventListener("submit", function (event) {
  var form = event.target;
  if (form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    event.preventDefault();
  }

  if (!form.matches("[data-task-toggle]")) {
    return;
  }

  event.preventDefault();

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
});

// Refleja en la interfaz si la tarea está completada.
function renderTaskCompleted(task, form, completed) {
  task.classList.toggle("task-completed", completed);
  var button = form.querySelector("button");
  var status = task.querySelector(".task-status");

  var label = completed ? "Reabrir" : "Completar";
  button.querySelector(".toggle-label").textContent = label;
  button.setAttribute("aria-pressed", String(completed));
  button.setAttribute("aria-label", label + " tarea: " + task.querySelector(".task-title").textContent);
  button.setAttribute("title", label + " tarea");

  if (completed && !status) {
    status = document.createElement("span");
    status.className = "task-status";
    status.textContent = "Completada";
    task.insertBefore(status, task.querySelector(".task-actions"));
  } else if (!completed && status) {
    status.remove();
  }
}
