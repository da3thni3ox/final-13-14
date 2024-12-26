package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
	// task.Date — это строка, которую нужно парсить в time.Time
	// taskDate, err := time.Parse("20060102", task.Date)
	// if err != nil {
	// 	res.Header().Set("Content-Type", "application/json")
	// 	res.WriteHeader(http.StatusInternalServerError)
	// 	json.NewEncoder(res).Encode(map[string]string{
	// 		"error": fmt.Sprintf("Ошибка парсинга даты: %v", err),
	// 	})
	// 	return
	// }

	// Получаем следующую дату для выполнения задачи
	now := time.Now()
	nextExecutionDate, err := NextDate(now, task.Date, task.Repeat)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка вычисления следующей даты: %v", err),
		})
		return
	}

	// nextExecutionDate должен быть типом time.Time, а не строкой
	parsedNextExecutionDate, err := time.Parse("20060102", nextExecutionDate)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(res).Encode(map[string]string{
			"error": fmt.Sprintf("Ошибка парсинга следующей даты: %v", err),
		})
		return
	}

	// Обновляем задачу с новой датой
	_, err = db.Exec(`
		UPDATE scheduler
		SET date = ?
		WHERE id = ?;
	`, parsedNextExecutionDate.Format("20060102"), id)
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

func apiNextDateHandler(res http.ResponseWriter, req *http.Request) {
	nowStr := req.FormValue("now")
	dateStr := req.FormValue("date")
	repeat := req.FormValue("repeat")

	const layout = "20060102"
	now, err := time.Parse(layout, nowStr)
	if err != nil {
		http.Error(res, "invalid 'now' parameter", http.StatusBadRequest)
		return
	}

	nextDate, err := NextDate(now, dateStr, repeat)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]string{
			"error": "",
		})
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write([]byte(nextDate))
}
