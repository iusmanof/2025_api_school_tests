package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// Database setup
var db *sqlx.DB

// Initialize the database connection
func InitDB(connectionString string) error {
	var err error
	db, err = sqlx.Open("postgres", connectionString)
	if err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS students (student_id SERIAL PRIMARY KEY, name TEXT)")
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS questions (question_id SERIAL PRIMARY KEY, class_id INT, question_text TEXT NOT NULL, options TEXT[], correct_answer TEXT)")
	if err != nil {
		return fmt.Errorf("could not create tables: %w", err)
	}

	return nil
}

// Insert the question data into the database
func InsertQuestion(id_cl int, q string, options []string, cor string) error {
	_, err := db.Exec("INSERT INTO questions (class_id, question_text, options, correct_answer) VALUES ($1, $2, $3, $4)",
		id_cl, q, pq.Array(options), cor)
	if err != nil {
		log.Printf("Error inserting question: %v", err)
		return fmt.Errorf("failed to insert question: %w", err)
	}
	return nil
}

// Delete all data from the questions table
func DeleteAllQuestions() error {
	_, err := db.Exec("DELETE FROM questions")
	if err != nil {
		log.Printf("Error deleting all questions: %v", err)
		return fmt.Errorf("failed to delete all questions: %w", err)
	}
	return nil
}

func CloseDB() {
	db.Close()
}

// Service logic for processing the CSV
func ProcessCSVFile(r *http.Request) error {
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return fmt.Errorf("could not read file: %w", err)
	}
	defer file.Close()

	// Parse the CSV
	reader := csv.NewReader(file)
	reader.Comma = ',' // Ensure that we are using the correct delimiter for CSV

	// Read all CSV records
	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("Error reading CSV file: %v", err)
		return fmt.Errorf("could not read CSV file: %w", err)
	}

	if len(records) > 0 {
		records = records[1:]
	}

	log.Printf("CSV records read: %v", len(records))

	// Insert each row from the CSV into the database
	for i, record := range records {
		err := insertQuestionData(record)
		if err != nil {
			log.Printf("Error inserting row %d: %v", i, err)
			return fmt.Errorf("error inserting data: %w", err)
		}
	}

	return nil
}

func insertQuestionData(record []string) error {
	if len(record) != 7 {
		return errors.New("invalid CSV format")
	}

	id_cl := record[0]
	q := record[1]
	ans1 := record[2]
	ans2 := record[3]
	ans3 := record[4]
	ans4 := record[5]
	cor := record[6]

	log.Printf("Processing record: %v", record)

	// Convert id_cl to integer
	id_cl_int, err := strconv.Atoi(id_cl)
	if err != nil {
		log.Printf("Invalid id_cl value: %v", err)
		return fmt.Errorf("invalid id_cl value: %v", err)
	}

	options := []string{ans1, ans2, ans3, ans4}

	// Insert question into the questions table
	err = InsertQuestion(id_cl_int, q, options, cor)
	if err != nil {
		log.Printf("Failed to insert question: %v", err)
		return fmt.Errorf("failed to insert question: %w", err)
	}

	return nil
}

// Controller to handle the file upload
func UploadCSVHandler(w http.ResponseWriter, r *http.Request) {
	err := ProcessCSVFile(r)
	if err != nil {
		log.Printf("Error in processing CSV: %v", err)
		http.Error(w, "Error uploading CSV file", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("CSV uploaded successfully"))
}

// Controller to delete all questions
func DeleteAllQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	err := DeleteAllQuestions()
	if err != nil {
		log.Printf("Error in deleting all questions: %v", err)
		http.Error(w, "Error deleting all questions", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("All questions deleted successfully"))
}

// Register the routes
func RegisterUploadRoutes(router *mux.Router) {
	router.HandleFunc("/upload-csv", UploadCSVHandler).Methods("POST")
	router.HandleFunc("/delete-all-questions", DeleteAllQuestionsHandler).Methods("POST") // Add route for deleting questions
	router.HandleFunc("/get-test-questions", GetTestQuestionsHandler).Methods("GET")
	router.HandleFunc("/submit-answers", SubmitAnswersHandler).Methods("POST")
}

func GetTestQuestions(classID int) ([]Question, error) {
	var questions []Question
	query := "SELECT question_id, question_text, options, correct_answer FROM questions WHERE class_id = $1 ORDER BY RANDOM() LIMIT 10"
	log.Printf("Executing query: %s with classID: %d", query, classID)
	err := db.Select(&questions, query, classID)
	if err != nil {
		return nil, fmt.Errorf("error fetching questions: %w", err)
	}
	return questions, nil
}

// Struct to represent the question
type Question struct {
	QuestionID    int            `json:"question_id" db:"question_id"`
	QuestionText  string         `json:"question_text" db:"question_text"`
	Options       pq.StringArray `json:"options" db:"options"` // Используем pq.StringArray для массива строк
	CorrectAnswer string         `json:"correct_answer" db:"correct_answer"`
}

func GetTestQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	classID := r.URL.Query().Get("class")
	if classID == "" {
		http.Error(w, "Class ID is missing", http.StatusBadRequest)
		return
	}

	classIDInt, err := strconv.Atoi(classID)
	if err != nil {
		http.Error(w, "Invalid class ID", http.StatusBadRequest)
		return
	}

	// Fetch the questions for the given class
	questions, err := GetTestQuestions(classIDInt)
	if err != nil {
		log.Printf("Error fetching questions: %v", err) // Log the error to the server logs
		http.Error(w, fmt.Sprintf("Error fetching questions: %v", err), http.StatusInternalServerError)
		return
	}

	// If no questions are found, return a custom error
	if len(questions) == 0 {
		http.Error(w, "No questions found for the specified class", http.StatusNotFound)
		return
	}

	// If questions are found, send them as a JSON response
	response := map[string]interface{}{
		"success":   true,
		"questions": questions,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func SubmitAnswersHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Answers []string `json:"answers"`
	}

	// Decode the submitted answers
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, "Invalid answer format", http.StatusBadRequest)
		return
	}

	classID := r.URL.Query().Get("class")
	classIDInt, err := strconv.Atoi(classID)
	if err != nil {
		http.Error(w, "Invalid class ID", http.StatusBadRequest)
		return
	}

	// Fetch the correct answers from the database
	var correctAnswers []string
	err = db.Select(&correctAnswers, "SELECT correct_answer FROM questions WHERE class_id = $1 LIMIT 10", classIDInt)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching correct answers: %v", err), http.StatusInternalServerError)
		return
	}

	// Compare the answers
	correctCount := 0
	for i, answer := range input.Answers {
		if answer == correctAnswers[i] {
			correctCount++
		}
	}

	// Return the result
	result := map[string]interface{}{
		"correct": correctCount,
		"total":   10,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Main function to run the application
func main() {
	err := InitDB(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Failed to initialize the database: ", err)
	}
	defer CloseDB()

	router := mux.NewRouter()

	fs := http.FileServer(http.Dir("static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	// Serve the index.html as the home page
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})

	RegisterUploadRoutes(router)

	log.Fatal(http.ListenAndServe(":8000", router))
}
