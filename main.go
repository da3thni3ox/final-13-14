package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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

func initDb(config config) error {
	var migration bool

	// Проверяем, существует ли файл базы данных
	if _, err := os.Stat(config.DbFilePath); os.IsNotExist(err) {
		migration = true
	}

	// Открываем базу данных
	var err error
	db, err = sql.Open("sqlite", config.DbFilePath)
	if err != nil {
		return fmt.Errorf("ошибка открытия базы данных: %v", err)
	}

	if migration {
		// Создаём таблицу, если её нет
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS scheduler (
				id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
				date TEXT NOT NULL,
				title TEXT NOT NULL,
				comment TEXT,
				repeat TEXT CHECK (LENGTH(repeat) <= 128)
			);
		`)
		if err != nil {
			return fmt.Errorf("ошибка создания таблицы: %v", err)
		}

		// Создаём индекс для ускорения запросов
		_, err = db.Exec(`
			CREATE INDEX IF NOT EXISTS scheduler_date ON scheduler(date);
		`)
		if err != nil {
			return fmt.Errorf("ошибка создания индекса: %v", err)
		}
	}

	return nil
}

// nextDate вычисляет следующую дату для задачи в соответствии с правилом
func nextDate(currentDate time.Time, rule string) (time.Time, error) {
	parts := strings.Fields(rule)
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("правило не указано")
	}

	switch parts[0] {
	case "d": // Добавить дни
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("отсутствует количество дней в правиле: %s", rule)
		}
		var days int
		if _, err := fmt.Sscanf(parts[1], "%d", &days); err != nil || days < 1 || days > 400 {
			return time.Time{}, fmt.Errorf("неправильное правило: %s", rule)
		}
		return currentDate.AddDate(0, 0, days), nil

	case "y": // Добавить год
		return currentDate.AddDate(1, 0, 0), nil

	case "w": // Ближайший день недели
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("отсутствуют дни недели в правиле: %s", rule)
		}
		days := parseInts(parts[1], 1, 7)
		if len(days) == 0 {
			return time.Time{}, fmt.Errorf("неправильные дни недели: %s", parts[1])
		}
		currentDay := int(currentDate.Weekday())
		if currentDay == 0 {
			currentDay = 7 // Преобразуем воскресенье в 7
		}
		for diff := 1; diff <= 7; diff++ {
			for _, day := range days {
				if (currentDay+diff-1)%7+1 == day {
					return currentDate.AddDate(0, 0, diff), nil
				}
			}
		}

	case "m": // Дни и месяцы
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("отсутствуют дни или месяцы в правиле: %s", rule)
		}
		days, months := parseMonthlyRule(parts[1:])
		if len(days) == 0 {
			return time.Time{}, fmt.Errorf("неправильное правило: %s", rule)
		}
		for {
			if len(months) == 0 || contains(months, int(currentDate.Month())) {
				for _, day := range days {
					candidate := resolveDay(currentDate.Year(), int(currentDate.Month()), day)
					if candidate.After(currentDate) {
						return candidate, nil
					}
				}
			}
			currentDate = currentDate.AddDate(0, 1, 0) // Следующий месяц
		}

	default:
		return time.Time{}, fmt.Errorf("неизвестное правило: %s", rule)
	}
	return time.Time{}, nil
}

// parseInts парсит список чисел из строки
func parseInts(input string, min, max int) []int {
	parts := strings.Split(input, ",")
	var result []int
	for _, part := range parts {
		var value int
		if _, err := fmt.Sscanf(part, "%d", &value); err == nil && value >= min && value <= max {
			result = append(result, value)
		}
	}
	return result
}

// parseMonthlyRule парсит дни и месяцы из правила
func parseMonthlyRule(parts []string) ([]int, []int) {
	days := parseInts(parts[0], -31, 31)
	months := []int{}
	if len(parts) > 1 {
		months = parseInts(parts[1], 1, 12)
	}
	return days, months
}

// resolveDay возвращает корректную дату для указанного дня месяца
func resolveDay(year, month, day int) time.Time {
	if day > 0 {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(year, time.Month(month), lastDay+day+1, 0, 0, 0, 0, time.UTC)
}

// contains проверяет, содержится ли элемент в списке
func contains(list []int, elem int) bool {
	for _, v := range list {
		if v == elem {
			return true
		}
	}
	return false
}

func updateTaskInDB(task Task) error {
	// Парсим дату задачи
	var taskDate time.Time
	var err error

	if task.Date != "" {
		// Если дата указана, пытаемся её распарсить
		taskDate, err = time.Parse("20060102", task.Date)
		if err != nil {
			return fmt.Errorf("неправильный формат даты, ожидается YYYYMMDD: %v", err)
		}
	} else {
		// Если дата не указана, используем текущую
		taskDate = time.Now()
	}

	// Проверяем правило повторения
	if task.Repeat != "" {
		_, err := nextDate(taskDate, task.Repeat)
		if err != nil {
			return fmt.Errorf("не удалось вычислить следующую дату выполнения: %v", err)
		}
	}

	// Обновляем задачу в базе данных
	query := `
		UPDATE scheduler
		SET date = ?, title = ?, comment = ?, repeat = ?
		WHERE id = ?;
	`
	result, err := db.Exec(query, taskDate.Format("20060102"), task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		return fmt.Errorf("ошибка обновления задачи: %v", err)
	}

	// Проверяем, было ли обновлено хотя бы одно значение
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка проверки обновления: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("задача с id=%s не найдена", task.ID)
	}

	return nil
}

func saveTaskToDB(task Task) (int64, error) {
	// Парсим дату задачи
	var taskDate time.Time
	var err error

	// Если дата не указана, подставляем сегодняшнюю
	if task.Date == "" {
		taskDate = time.Now()
	} else {
		// Если дата указана, пытаемся её распарсить
		taskDate, err = time.Parse("20060102", task.Date)
		if err != nil {
			return -1, fmt.Errorf("неправильный формат даты, ожидается YYYYMMDD: %v", err)
		}
	}

	// Если дата меньше сегодняшнего дня, подставляем текущую дату
	if taskDate.Before(time.Now()) {
		taskDate = time.Now()
	}

	// Если правило повторения пустое или отсутствует, дата остаётся сегодняшней
	if task.Repeat == "" {
		// Ничего не делаем, сохраняем текущую дату
	} else {
		// Проверяем правило повторения для валидации
		_, err := nextDate(taskDate, task.Repeat)
		if err != nil {
			return -1, fmt.Errorf("не удалось вычислить следующую дату выполнения: %v", err)
		}
	}

	// Сохраняем задачу в базу данных
	query := `
	INSERT INTO scheduler (date, title, comment, repeat) 
	VALUES (?, ?, ?, ?);
	`
	result, err := db.Exec(query, taskDate.Format("20060102"), task.Title, task.Comment, task.Repeat)
	if err != nil {
		return 0, fmt.Errorf("ошибка сохранения задачи: %v", err)
	}

	// Получаем ID последней вставленной записи
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка получения ID задачи: %v", err)
	}

	return id, nil
}

func handleMain(res http.ResponseWriter, req *http.Request) {
	fs := http.FileServer(http.Dir("./web"))
	http.StripPrefix("/", fs).ServeHTTP(res, req)
}

func handleTask(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {

		// Получаем ID из запроса
		id := req.URL.Query().Get("id")
		if id == "" {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Не указан идентификатор задачи",
			})
			return
		}

		// Выполняем запрос к базе данных
		query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?`
		var task struct {
			ID      string `json:"id"`
			Date    string `json:"date"`
			Title   string `json:"title"`
			Comment string `json:"comment"`
			Repeat  string `json:"repeat"`
		}
		err := db.QueryRow(query, id).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			if err == sql.ErrNoRows {
				res.Header().Set("Content-Type", "application/json")
				res.WriteHeader(http.StatusNotFound)
				json.NewEncoder(res).Encode(map[string]string{
					"error": "Задача не найдена",
				})
				return
			}
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": fmt.Sprintf("Ошибка выполнения запроса: %v", err),
			})
			return
		}

		// Возвращаем задачу
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(task)
	}

	if req.Method == http.MethodPut {
		var task Task
		if err := json.NewDecoder(req.Body).Decode(&task); err != nil {
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Неверный формат JSON",
			})
			return
		}

		if task.ID == "" {
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Поле 'id' является обязательным",
			})
			return
		}

		if task.Title == "" {
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Поле 'title' является обязательным",
			})
			return
		}

		err := updateTaskInDB(task)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(map[string]any{})
	}

	if req.Method == http.MethodPost {

		var task Task
		if err := json.NewDecoder(req.Body).Decode(&task); err != nil {
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Неверный формат JSON",
			})
			return
		}

		if task.Title == "" {
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Поле 'title' является обязательным",
			})
			return
		}

		id, err := saveTaskToDB(task)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		res.WriteHeader(http.StatusCreated)
		json.NewEncoder(res).Encode(map[string]any{
			"id": id,
		})
	}

	if req.Method == http.MethodDelete {

		id := req.URL.Query().Get("id")
		if id == "" {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Не указан идентификатор задачи",
			})
			return
		}

		// Выполняем запрос к базе данных для получения информации о задаче
		query := `SELECT id FROM scheduler WHERE id = ?`
		var taskID string
		err := db.QueryRow(query, id).Scan(&taskID)
		if err != nil {
			if err == sql.ErrNoRows {
				res.Header().Set("Content-Type", "application/json")
				res.WriteHeader(http.StatusNotFound)
				json.NewEncoder(res).Encode(map[string]string{
					"error": "Задача не найдена",
				})
				return
			}
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": fmt.Sprintf("Ошибка выполнения запроса: %v", err),
			})
			return
		}

		// Удаляем задачу из базы данных
		_, err = db.Exec(`DELETE FROM scheduler WHERE id = ?`, id)
		if err != nil {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": fmt.Sprintf("Ошибка удаления задачи: %v", err),
			})
			return
		}

		// Возвращаем пустой JSON в случае успешного удаления
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(map[string]any{})

	}

}

func handleGetTasks(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(res).Encode(map[string]string{
			"error": "Метод не поддерживается",
		})
		return
	}

	query := `
        SELECT id, date, title, comment, repeat
        FROM scheduler
        ORDER BY date ASC
        LIMIT 50;
    `
	rows, err := db.Query(query)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка получения задач: %v", err),
		})
		return
	}
	defer rows.Close()

	var tasks []map[string]string
	for rows.Next() {
		var id, date, title, comment, repeat string
		if err := rows.Scan(&id, &date, &title, &comment, &repeat); err != nil {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": fmt.Sprintf("Ошибка чтения данных: %v", err),
			})
			return
		}
		tasks = append(tasks, map[string]string{
			"id":      id,
			"date":    date,
			"title":   title,
			"comment": comment,
			"repeat":  repeat,
		})
	}

	if len(tasks) == 0 {
		tasks = []map[string]string{} // Возвращаем пустой список вместо nil
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]interface{}{
		"tasks": tasks,
	})
}

func handleTaskDone(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(res).Encode(map[string]string{
			"error": "Метод не поддерживается",
		})
		return
	}

	// Получаем идентификатор из запроса
	id := req.URL.Query().Get("id")
	if id == "" {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(res).Encode(map[string]string{
			"error": "Не указан идентификатор задачи",
		})
		return
	}

	// Выполняем запрос к базе данных для получения информации о задаче
	query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?`
	var task Task
	err := db.QueryRow(query, id).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err != nil {
		if err == sql.ErrNoRows {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusNotFound)
			json.NewEncoder(res).Encode(map[string]string{
				"error": "Задача не найдена",
			})
			return
		}
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка выполнения запроса: %v", err),
		})
		return
	}

	// Если задача одноразовая (с пустым repeat), удаляем её
	if task.Repeat == "" {
		// Удаляем задачу из базы данных
		_, err := db.Exec(`DELETE FROM scheduler WHERE id = ?`, id)
		if err != nil {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(res).Encode(map[string]string{
				"error": fmt.Sprintf("Ошибка удаления задачи: %v", err),
			})
			return
		}
		// Возвращаем пустой JSON
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(map[string]any{})
		return
	}

	// Если задача периодическая, рассчитываем следующую дату выполнения
	taskDate, err := time.Parse("20060102", task.Date)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка парсинга даты: %v", err),
		})
		return
	}

	nextExecutionDate, err := nextDate(taskDate, task.Repeat)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка вычисления следующей даты: %v", err),
		})
		return
	}

	// Обновляем задачу с новой датой
	_, err = db.Exec(`
		UPDATE scheduler
		SET date = ?
		WHERE id = ?;
	`, nextExecutionDate.Format("20060102"), id)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка обновления даты задачи: %v", err),
		})
		return
	}

	// Возвращаем пустой JSON в случае успешного выполнения
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]any{})
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
