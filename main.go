package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	_ "modernc.org/sqlite"
)

type config struct {
	ListenAddress string
	ListenPort    string
	DbFilePath    string
}

type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

var db *sql.DB // Глобальная переменная для доступа к базе данных

func loadConfig() config {
	return config{
		ListenAddress: getenv("TODO_LISTEN_ADDRESS", "127.0.0.1"),
		ListenPort:    getenv("TODO_PORT", "8080"),
		DbFilePath:    getenv("TODO_DBFILE_PATH", "./tasks.db"),
	}
}

func getenv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	config := loadConfig()

	if err := initDb(config); err != nil {
		fmt.Println("Ошибка инициализации базы данных:", err)
		return
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleMain)
	mux.HandleFunc("/api/task", handleTask)
	mux.HandleFunc("/api/tasks", handleGetTasks)
	mux.HandleFunc("/api/task/done", handleTaskDone)

	fmt.Printf("Сервер запущен на http://%s:%s\n", config.ListenAddress, config.ListenPort)
	if err := http.ListenAndServe(config.ListenAddress+":"+config.ListenPort, mux); err != nil {
		panic(err)
	}
}
