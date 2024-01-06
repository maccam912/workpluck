package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

// Task represents a task with an ID, topic, and input data.
type Task struct {
	ID    string      `json:"id"`
	Topic string      `json:"topic"`
	Input interface{} `json:"input"`
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

// handleTaskSubmit for submitting a new task.
func handleTaskSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task.ID = uuid.New().String() // Generate a new UUID for the task

	storeMutex.Lock()
	taskStore[task.ID] = task
	storeMutex.Unlock()

	w.WriteHeader(http.StatusCreated) // Set the status code to 201 Created
	json.NewEncoder(w).Encode(map[string]string{"id": task.ID})
}

func handleRetrieveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "Topic is required", http.StatusBadRequest)
		return
	}

	storeMutex.Lock()
	defer storeMutex.Unlock()

	for _, task := range taskStore {
		if task.Topic == topic {
			json.NewEncoder(w).Encode(task)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent) // No task available
}

func handleSubmitResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var result Result
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	storeMutex.Lock()
	_, taskExists := taskStore[result.ID]
	if !taskExists {
		storeMutex.Unlock()
		http.Error(w, "Task does not exist", http.StatusNotFound)
		return
	}

	resultStore[result.ID] = result
	storeMutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func handleGetResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	storeMutex.Lock()
	defer storeMutex.Unlock()

	result, resultExists := resultStore[id]
	_, taskExists := taskStore[id]

	if !taskExists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if !resultExists {
		w.WriteHeader(http.StatusAccepted) // Task exists but is not yet completed
		return
	}

	json.NewEncoder(w).Encode(result)
}

func handleTask(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handleTaskSubmit(w, r)
	case http.MethodGet:
		handleRetrieveTask(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleResult(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handleSubmitResult(w, r)
	case http.MethodGet:
		handleGetResult(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	http.HandleFunc("/task", handleTask)
	http.HandleFunc("/result", handleResult)

	fmt.Println("Server is starting on port 8080...")
	http.ListenAndServe(":8080", nil)
}
