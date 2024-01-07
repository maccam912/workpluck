package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleTaskSubmit tests the task submission handler.
func TestHandleTaskSubmit(t *testing.T) {
	// Mock a request
	task := Task{Topic: "example", Input: map[string]string{"key": "value"}}
	taskJSON, _ := json.Marshal(task)
	req, err := http.NewRequest("POST", "/task", bytes.NewBuffer(taskJSON))
	if err != nil {
		t.Fatal(err)
	}

	// Record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTaskSubmit)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Check the response body
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if _, ok := response["id"]; !ok {
		t.Errorf("handler did not return a task ID")
	}
}

// TestHandleRetrieveTask tests the task retrieval handler.
func TestHandleRetrieveTask(t *testing.T) {
	// Clear and pre-populate taskStore with a task
	taskStore = make(map[string]Task)
	testTask := Task{ID: "test-id", Topic: "test", Input: map[string]string{"key": "value"}, Status: "new"}
	taskStore[testTask.ID] = testTask

	// Mock a request to retrieve the task
	req, err := http.NewRequest("GET", "/task?topic=test", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRetrieveTask)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	var response Task
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != testTask.ID || response.Status != "pending" {
		t.Errorf("handler returned unexpected task: got ID %v with status %v, want ID %v with status pending", response.ID, response.Status, testTask.ID)
	}
}

// TestHandleSubmitResult tests the result submission handler.
func TestHandleSubmitResult(t *testing.T) {
	// Clear taskStore and resultStore, then pre-populate taskStore with a task
	taskStore = make(map[string]Task)
	resultStore = make(map[string]Result)
	testTask := Task{ID: "test-id", Topic: "test", Input: map[string]string{"key": "value"}, Status: "pending"}
	taskStore[testTask.ID] = testTask

	// Mock a request to submit a result
	result := Result{ID: testTask.ID, Output: map[string]string{"result": "success"}}
	resultJSON, _ := json.Marshal(result)
	req, err := http.NewRequest("POST", "/result", bytes.NewBuffer(resultJSON))
	if err != nil {
		t.Fatal(err)
	}

	// Record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleSubmitResult)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify task status is updated
	updatedTask, exists := taskStore[testTask.ID]
	if !exists || updatedTask.Status != "completed" {
		t.Errorf("task status was not updated correctly: got %v, want completed", updatedTask.Status)
	}
}

// TestHandleGetResult tests the task result retrieval handler.
func TestHandleGetResult(t *testing.T) {
	// Pre-populate resultStore with a result
	testResult := Result{ID: "test-id", Output: map[string]string{"result": "success"}}
	resultStore[testResult.ID] = testResult

	// Mock a request to retrieve the result
	req, err := http.NewRequest("GET", "/result?id=test-id", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleGetResult)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	var response Result
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != testResult.ID {
		t.Errorf("handler returned unexpected result: got %v want %v", response.ID, testResult.ID)
	}
}

// TestEndToEnd tests the entire flow from task submission to result retrieval.
func TestEndToEnd(t *testing.T) {
	// Submit a task
	submitReq, _ := http.NewRequest("POST", "/task", bytes.NewBuffer([]byte(`{"topic": "test", "input": {"data": "test"}}`)))
	submitRr := httptest.NewRecorder()
	handleTaskSubmit(submitRr, submitReq)

	var submitResponse map[string]string
	json.Unmarshal(submitRr.Body.Bytes(), &submitResponse)
	taskID, ok := submitResponse["id"]
	if !ok {
		t.Fatalf("Task submission did not return an ID")
	}

	// Retrieve and submit the result for the task
	retrieveReq, _ := http.NewRequest("GET", "/task?topic=test", nil)
	retrieveRr := httptest.NewRecorder()
	handleRetrieveTask(retrieveRr, retrieveReq)

	submitResultReq, _ := http.NewRequest("POST", "/result", bytes.NewBuffer([]byte(`{"id": "`+taskID+`", "output": {"result": "success"}}`)))
	submitResultRr := httptest.NewRecorder()
	handleSubmitResult(submitResultRr, submitResultReq)

	if submitResultRr.Code != http.StatusOK {
		t.Fatalf("Submitting result failed, got status code %d", submitResultRr.Code)
	}

	// Retrieve the result
	getResultReq, _ := http.NewRequest("GET", "/result?id="+taskID, nil)
	getResultRr := httptest.NewRecorder()
	handleGetResult(getResultRr, getResultReq)

	if getResultRr.Code != http.StatusOK {
		t.Fatalf("Retrieving result failed, got status code %d", getResultRr.Code)
	}
}
