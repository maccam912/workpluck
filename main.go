package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Task represents a task with an ID, topic, and input data.
type Task struct {
	ID        string      `json:"id"`
	Topic     string      `json:"topic"`
	Input     interface{} `json:"input"`
	Status    string      `json:"status"`    // "new", "pending", "completed"
	Timestamp time.Time   `json:"timestamp"` // Time when the task was retrieved
}

// Result represents the output of a processed task.
type Result struct {
	ID     string      `json:"id"`
	Output interface{} `json:"output"`
}

// taskStore holds the submitted tasks.
var taskStore = make(map[string]Task)

// resultStore holds the results of processed tasks.
var resultStore = make(map[string]Result)

// mutex for concurrent access to the stores.
var storeMutex = &sync.Mutex{}

var tracer = otel.GetTracerProvider().Tracer("TaskServer")

func initTracer() {
	// exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	// if err != nil {
	// 	log.Fatalf("Failed to initialize stdouttrace exporter: %v", err)
	// }
	tp := trace.NewTracerProvider(
		// trace.WithBatcher(exp),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			attribute.String("service.name", "TaskService"),
		)),
	)
	otel.SetTracerProvider(tp)
}

func handleTaskSubmit(w http.ResponseWriter, r *http.Request) {
	log.Println("handleTaskSubmit called")
	_, span := tracer.Start(r.Context(), "handleTaskSubmit")
	defer span.End()

	if r.Method != http.MethodPost {
		log.Println("Invalid method in handleTaskSubmit")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.AddEvent("Invalid method")
		return
	}

	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		log.Printf("Error decoding task: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		span.RecordError(err)
		return
	}

	task.ID = uuid.New().String()
	task.Status = "new"
	task.Timestamp = time.Now()

	storeMutex.Lock()
	taskStore[task.ID] = task
	storeMutex.Unlock()

	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(map[string]string{"id": task.ID})
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		span.RecordError(err)
	}
	log.Printf("Task submitted: %s", task.ID)
}

func handleRetrieveTask(w http.ResponseWriter, r *http.Request) {
	log.Println("handleRetrieveTask called")
	_, span := tracer.Start(r.Context(), "handleRetrieveTask")
	defer span.End()

	if r.Method != http.MethodGet {
		log.Println("Invalid method in handleRetrieveTask")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.AddEvent("Invalid method")
		return
	}

	topic := r.URL.Query().Get("topic")
	if topic == "" {
		log.Println("Topic is required in handleRetrieveTask")
		http.Error(w, "Topic is required", http.StatusBadRequest)
		span.AddEvent("Missing topic")
		return
	}

	storeMutex.Lock()
	defer storeMutex.Unlock()

	currentTime := time.Now()
	for id, task := range taskStore {
		if task.Topic == topic && (task.Status == "new" || (task.Status == "pending" && currentTime.Sub(task.Timestamp) > time.Hour)) {
			task.Status = "pending"
			task.Timestamp = currentTime
			taskStore[id] = task
			err := json.NewEncoder(w).Encode(task)
			if err != nil {
				log.Printf("Error encoding task: %v", err)
				span.RecordError(err)
			}
			log.Printf("Task retrieved: %s", task.ID)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
	log.Println("No task available in handleRetrieveTask")
}

func handleSubmitResult(w http.ResponseWriter, r *http.Request) {
	log.Println("handleSubmitResult called")
	_, span := tracer.Start(r.Context(), "handleSubmitResult")
	defer span.End()

	if r.Method != http.MethodPost {
		log.Println("Invalid method in handleSubmitResult")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.AddEvent("Invalid method")
		return
	}

	var result Result
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Printf("Error decoding result: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		span.RecordError(err)
		return
	}

	storeMutex.Lock()
	task, taskExists := taskStore[result.ID]
	if !taskExists {
		storeMutex.Unlock()
		log.Printf("Task does not exist: %s", result.ID)
		http.Error(w, "Task does not exist", http.StatusNotFound)
		span.AddEvent("Task not found")
		return
	}

	task.Status = "completed"
	taskStore[result.ID] = task
	resultStore[result.ID] = result
	storeMutex.Unlock()

	w.WriteHeader(http.StatusOK)
	log.Printf("Result submitted for task: %s", result.ID)
}

func handleGetResult(w http.ResponseWriter, r *http.Request) {
	log.Println("handleGetResult called")
	_, span := tracer.Start(r.Context(), "handleGetResult")
	defer span.End()

	if r.Method != http.MethodGet {
		log.Println("Invalid method in handleGetResult")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.AddEvent("Invalid method")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		log.Println("ID is required in handleGetResult")
		http.Error(w, "ID is required", http.StatusBadRequest)
		span.AddEvent("Missing ID")
		return
	}

	storeMutex.Lock()
	defer storeMutex.Unlock()

	result, resultExists := resultStore[id]
	_, taskExists := taskStore[id]

	if !taskExists {
		log.Printf("Task not found: %s", id)
		http.Error(w, "Task not found", http.StatusNotFound)
		span.AddEvent("Task not found")
		return
	}

	if !resultExists {
		w.WriteHeader(http.StatusAccepted)
		log.Printf("Task exists but result not completed: %s", id)
		return
	}

	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		log.Printf("Error encoding result: %v", err)
		span.RecordError(err)
	}
	log.Printf("Result retrieved for task: %s", result.ID)
}

func handleTask(w http.ResponseWriter, r *http.Request) {
	log.Println("handleTask called")
	switch r.Method {
	case http.MethodPost:
		handleTaskSubmit(w, r)
	case http.MethodGet:
		handleRetrieveTask(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Println("Invalid method in handleTask")
	}
}

func handleResult(w http.ResponseWriter, r *http.Request) {
	log.Println("handleResult called")
	switch r.Method {
	case http.MethodPost:
		handleSubmitResult(w, r)
	case http.MethodGet:
		handleGetResult(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Println("Invalid method in handleResult")
	}
}

func handleObserve(w http.ResponseWriter, r *http.Request) {
	log.Println("handleObserve called")
	_, span := tracer.Start(r.Context(), "handleObserve")
	defer span.End()

	// Print out entire contents of taskStore and resultStore
	storeMutex.Lock()
	defer storeMutex.Unlock()

	for id, task := range taskStore {
		log.Printf("Task: %s, %s, %s", id, task, task.Status)
		w.Write([]byte("Task: " + id + ", " + task.Status + "\n"))
	}
	for id, result := range resultStore {
		log.Printf("Result: %s, %s", id, result)
		w.Write([]byte("Result: " + id + ", " + result.ID + "\n"))
	}
}

func main() {
	initTracer()
	log.Println("Server is starting on port 8080...")
	http.HandleFunc("/task", handleTask)
	http.HandleFunc("/result", handleResult)
	http.HandleFunc("/observe", handleObserve)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
