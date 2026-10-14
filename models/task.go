package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TitleMaxLength es el número máximo de caracteres permitido en el título.
const TitleMaxLength = 200

// Prioridades permitidas para una tarea. PriorityMedium es también el valor
// por defecto de la columna en la migración; si cambia, deben cambiar ambos.
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
func PriorityOptions() []PriorityOption {
	return []PriorityOption{
		{Value: PriorityHigh, Label: "Alta"},
		{Value: PriorityMedium, Label: "Media"},
		{Value: PriorityLow, Label: "Baja"},
	}
}

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID `json:"id" db:"id" form:"-"`
	Title     string    `json:"title" db:"title" form:"title"`
	Priority  string    `json:"priority" db:"priority" form:"priority"`
	Completed bool      `json:"completed" db:"completed" form:"-"`
	CreatedAt time.Time `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" form:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// Validate se ejecuta en cada llamada a los métodos "pop.Validate*".
func (t *Task) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()

	title := strings.TrimSpace(t.Title)
	if title == "" {
		verrs.Add("title", "El título es obligatorio.")
	} else if len([]rune(title)) > TitleMaxLength {
		verrs.Add("title", fmt.Sprintf("El título no puede superar los %d caracteres.", TitleMaxLength))
	}

	// La prioridad vacía es válida: BeforeSave le asigna el valor por defecto.
	if t.Priority != "" && !isValidPriority(t.Priority) {
		verrs.Add("priority", "La prioridad no es válida.")
	}

	return verrs, nil
}

func isValidPriority(priority string) bool {
	for _, opt := range PriorityOptions() {
		if opt.Value == priority {
			return true
		}
	}
	return false
}

// BeforeSave asigna la prioridad por defecto cuando no se ha indicado ninguna.
func (t *Task) BeforeSave(tx *pop.Connection) error {
	if t.Priority == "" {
		t.Priority = PriorityMedium
	}
	return nil
}

// PriorityLabel devuelve la etiqueta en español de la prioridad, o el valor
// tal cual si no es una prioridad conocida.
func (t Task) PriorityLabel() string {
	for _, opt := range PriorityOptions() {
		if opt.Value == t.Priority {
			return opt.Label
		}
	}
	return t.Priority
}

// CreatedDateLabel devuelve la fecha de creación en formato dd/mm/aaaa.
func (t Task) CreatedDateLabel() string {
	return t.CreatedAt.Format("02/01/2006")
}

// CreatedDateISO devuelve la fecha de creación en formato aaaa-mm-dd, válido
// para el atributo datetime de <time>.
func (t Task) CreatedDateISO() string {
	return t.CreatedAt.Format("2006-01-02")
}
