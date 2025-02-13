package main

import (
	"fmt"
	"os"
)

func main() {
	fileName := "test1.csv"
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	fmt.Printf("file size %d\n", len(data))
	fmt.Printf("file content \n%s", data)

	// for loop by \n
	//    INSERT
	// 	  INSERT INTO questions (question_id SERIAL PRIMARY KEY, class_id INT, question_text TEXT NOT NULL,  options TEXT[], correct_answer TEXT) VALUES
	//   (1, 'What is 2 + 2?');

	// Generate Random Text Strings in PostgreSQL
}
