package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type Student struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

var students = make(map[int]Student)
var idCounter = 1

func createStudent(w http.ResponseWriter, r *http.Request) {
	var student Student
	if err := json.NewDecoder(r.Body).Decode(&student); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	student.ID = idCounter
	idCounter++
	students[student.ID] = student
	json.NewEncoder(w).Encode(student)
}

func getAllStudents(w http.ResponseWriter, r *http.Request) {
	studentList := []Student{}
	for _, student := range students {
		studentList = append(studentList, student)
	}
	json.NewEncoder(w).Encode(studentList)
}

func getStudentByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	if student, exists := students[id]; exists {
		json.NewEncoder(w).Encode(student)
	} else {
		http.Error(w, "Student not found", http.StatusNotFound)
	}
}

func updateStudent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	if student, exists := students[id]; exists {
		json.NewDecoder(r.Body).Decode(&student)
		student.ID = id
		students[id] = student
		json.NewEncoder(w).Encode(student)
	} else {
		http.Error(w, "Student not found", http.StatusNotFound)
	}
}

func deleteStudent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	if _, exists := students[id]; exists {
		delete(students, id)
		w.WriteHeader(http.StatusNoContent)
	} else {
		http.Error(w, "Student not found", http.StatusNotFound)
	}
}

func getStudentSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	student, exists := students[id]
	if !exists {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	// Prepare the prompt for Ollama
	prompt := fmt.Sprintf("Generate a summary for the student: Name: %s, Age: %d, Email: %s", student.Name, student.Age, student.Email)
	payload := map[string]interface{}{
		"model":  "llama3.2",
		"prompt": prompt,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to prepare request for Ollama", http.StatusInternalServerError)
		return
	}

	// Call the Ollama API
	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(payloadBytes))
	if err != nil {
		http.Error(w, "Failed to create request for Ollama", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to communicate with Ollama", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check for successful response status
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Ollama returned an error response", resp.StatusCode)
		return
	}

	// Read and parse the response
	decoder := json.NewDecoder(resp.Body)
	var completeResponse strings.Builder

	for {
		var result map[string]interface{}
		if err := decoder.Decode(&result); err == io.EOF {
			break // End of response
		} else if err != nil {
			http.Error(w, "Failed to parse response from Ollama", http.StatusInternalServerError)
			return
		}

		// Check for the response content and append it
		if responsePart, ok := result["response"].(string); ok {
			completeResponse.WriteString(responsePart)
		}
	}

	// Final summary output
	if completeResponse.Len() == 0 {
		http.Error(w, "No summary found in Ollama response", http.StatusInternalServerError)
		return
	}

	// Send the complete summary as JSON response
	json.NewEncoder(w).Encode(map[string]string{"summary": completeResponse.String()})
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/students", createStudent).Methods("POST")
	router.HandleFunc("/students", getAllStudents).Methods("GET")
	router.HandleFunc("/students/{id}", getStudentByID).Methods("GET")
	router.HandleFunc("/students/{id}", updateStudent).Methods("PUT")
	router.HandleFunc("/students/{id}", deleteStudent).Methods("DELETE")
	router.HandleFunc("/students/{id}/summary", getStudentSummary).Methods("GET")

	http.ListenAndServe(":8081", router)
}
